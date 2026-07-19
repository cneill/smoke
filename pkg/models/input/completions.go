package input

import (
	"fmt"
	"strings"
	"unicode/utf8"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/cneill/smoke/pkg/fs"
	"github.com/cneill/smoke/pkg/llmctx/skills"
)

type CompletionLeader byte

const (
	CompletionLeaderNone    CompletionLeader = 0
	CompletionLeaderCommand CompletionLeader = '/'
	CompletionLeaderSkill   CompletionLeader = '$'
	CompletionLeaderPath    CompletionLeader = '@'
)

const (
	completionPopupWindow = 4
	popupBorderLines      = 2
)

// Match is one selectable completion option shown in the popup.
type Match struct {
	// Value is used when filling/accepting. Path: full relative path. Command/skill: name only (no leader).
	Value string
	// Label is the display text in the popup (may include usage/description).
	Label string
}

// KeyResult describes how the input model should apply a completion key.
type KeyResult struct {
	// Consume is true when the key must not reach the textarea (navigation, accept, tab fill).
	Consume bool
	// Replace, when non-empty, replaces the active completion token in the textarea.
	Replace string
	// Leader is the token leader for ReplaceActiveToken ('/', '$', '@'). Required when Replace is set.
	Leader CompletionLeader
}

type CompletionState struct {
	maxWidth         int
	commandCompleter func(string) []string
	skillCompleter   func(string) []*skills.Skill
	pathCompleter    func(string) []fs.PathMatch

	completionLeader CompletionLeader
	userText         string // includes leader ("/h", "$sk", "@docs/")

	matches  []Match
	selected int
	window   int
}

func NewCompletionState(
	maxWidth int,
	commandCompleter func(string) []string,
	skillCompleter func(string) []*skills.Skill,
	pathCompleter func(string) []fs.PathMatch,
) (*CompletionState, error) {
	if commandCompleter == nil {
		return nil, fmt.Errorf("must supply command completer")
	}

	if skillCompleter == nil {
		return nil, fmt.Errorf("must supply skill completer")
	}

	if pathCompleter == nil {
		return nil, fmt.Errorf("must supply path completer")
	}

	return &CompletionState{
		maxWidth:         maxWidth,
		commandCompleter: commandCompleter,
		skillCompleter:   skillCompleter,
		pathCompleter:    pathCompleter,
		window:           completionPopupWindow,
	}, nil
}

func (c *CompletionState) CompletionLeader() CompletionLeader {
	return c.completionLeader
}

func (c *CompletionState) InCommandCompletion() bool {
	return c.completionLeader == CompletionLeaderCommand
}

func (c *CompletionState) InSkillCompletion() bool {
	return c.completionLeader == CompletionLeaderSkill
}

func (c *CompletionState) InPathCompletion() bool {
	return c.completionLeader == CompletionLeaderPath
}

func (c *CompletionState) InCompletion() bool {
	return c.completionLeader != CompletionLeaderNone
}

func (c *CompletionState) SetMaxWidth(maxWidth int) {
	c.maxWidth = maxWidth
}

// PopupActive reports whether the completion popup should be shown.
func (c *CompletionState) PopupActive() bool {
	if !c.InCompletion() || len(c.matches) == 0 {
		return false
	}

	// Path requires @ plus at least one additional character.
	if c.InPathCompletion() && len(c.userText) < 2 {
		return false
	}

	return true
}

// Matches returns the current match list (for rendering).
func (c *CompletionState) Matches() []Match {
	return c.matches
}

// Selected returns the selected index into Matches.
func (c *CompletionState) Selected() int {
	return c.selected
}

// VisibleRange returns the [start, end) window of matches to display.
func (c *CompletionState) VisibleRange() (int, int) {
	if len(c.matches) == 0 {
		return 0, 0
	}

	window := c.window
	if window <= 0 {
		window = completionPopupWindow
	}

	start := max(c.selected-(window-1), 0)

	end := start + window
	if end > len(c.matches) {
		end = len(c.matches)
		start = max(0, end-window)
	}

	return start, end
}

// PopupLineCount returns how many terminal lines the popup chrome occupies when active (0 if hidden).
// Height is derived from state, not from styled render output: items + optional help + border (2).
func (c *CompletionState) PopupLineCount() int {
	if !c.PopupActive() {
		return 0
	}

	start, end := c.VisibleRange()

	items := end - start
	if items <= 0 {
		return 0
	}

	lines := items
	if end-start < len(c.matches) {
		lines++ // help line when list is truncated
	}

	return lines + popupBorderLines
}

// HandleKey processes keys for command, skill, and path completion.
// If Consume is true the key must not reach the textarea.
func (c *CompletionState) HandleKey(msg tea.KeyMsg, currentText string) KeyResult { //nolint:cyclop,funlen
	if !c.InCompletion() {
		if !c.tryStart(msg, currentText) {
			return KeyResult{}
		}

		c.userText = msg.String()
		c.refreshMatches()

		return KeyResult{}
	}

	switch msg.Type { //nolint:exhaustive
	case tea.KeyEsc:
		c.Reset()

		return KeyResult{Consume: true}
	case tea.KeyEnter:
		if !c.PopupActive() {
			return KeyResult{}
		}

		leader := c.CompletionLeader()

		text := c.acceptText(c.matches[c.selected])
		c.Reset()

		return KeyResult{Consume: true, Replace: text, Leader: leader}
	case tea.KeyUp:
		if !c.PopupActive() {
			return KeyResult{}
		}

		if c.selected > 0 {
			c.selected--
		}

		return KeyResult{Consume: true}
	case tea.KeyDown:
		if !c.PopupActive() {
			return KeyResult{}
		}

		if c.selected < len(c.matches)-1 {
			c.selected++
		}

		return KeyResult{Consume: true}
	case tea.KeyTab:
		if !c.PopupActive() {
			return KeyResult{Consume: true}
		}

		selected := c.matches[c.selected]

		c.userText = c.tabUserText(selected)
		c.refreshMatches()

		return KeyResult{Consume: true, Replace: c.userText, Leader: c.CompletionLeader()}
	case tea.KeyBackspace:
		return c.handleBackspace()
	case tea.KeyRunes, tea.KeySpace:
		return c.handleRune(msg)
	default:
		// TODO: FIX MODIFICATIONS OF CURRENT USER TEXT, E.G. CHANGING @pkg/commands/ TO @pkg/confi
		return KeyResult{}
	}
}

func (c *CompletionState) Reset() {
	c.userText = ""
	c.completionLeader = CompletionLeaderNone
	c.matches = nil
	c.selected = 0
}

func (c *CompletionState) tryStart(msg tea.KeyMsg, currentText string) bool {
	key := msg.String()
	if key == "" {
		return false
	}

	switch CompletionLeader(key[0]) { //nolint:exhaustive // Not looking for a null byte for "None"
	case CompletionLeaderCommand:
		if currentText != "" {
			return false
		}

		c.completionLeader = CompletionLeaderCommand

		return true
	case CompletionLeaderSkill:
		if !atWordBoundary(currentText) {
			return false
		}

		c.completionLeader = CompletionLeaderSkill

		return true
	case CompletionLeaderPath:
		if !atWordBoundary(currentText) {
			return false
		}

		c.completionLeader = CompletionLeaderPath

		return true
	default:
		return false
	}
}

func (c *CompletionState) handleBackspace() KeyResult {
	if c.userText != "" {
		_, size := utf8.DecodeLastRuneInString(c.userText)
		if size <= 0 {
			size = 1
		}

		c.userText = c.userText[:len(c.userText)-size]
	}

	if c.userText == "" {
		c.Reset()
	} else {
		c.refreshMatches()
	}

	// let backspace also hit the textarea
	return KeyResult{}
}

func (c *CompletionState) handleRune(msg tea.KeyMsg) KeyResult {
	keyVal := msg.String()
	// whitespace ends the completion token
	if strings.ContainsAny(keyVal, "\t\n ") {
		c.Reset()

		return KeyResult{}
	}

	c.userText += keyVal
	c.refreshMatches()

	return KeyResult{} // let character reach textarea
}

func (c *CompletionState) refreshMatches() {
	c.selected = 0
	c.matches = nil

	if !c.InCompletion() {
		return
	}

	switch c.completionLeader {
	case CompletionLeaderCommand:
		prefix := strings.TrimPrefix(c.userText, string(CompletionLeaderCommand))
		c.matches = commandMatches(c.commandCompleter(prefix))
	case CompletionLeaderSkill:
		prefix := strings.TrimPrefix(c.userText, string(CompletionLeaderSkill))
		c.matches = c.skillMatches(c.skillCompleter(prefix))
	case CompletionLeaderPath:
		if len(c.userText) < 2 {
			return
		}

		query := strings.TrimPrefix(c.userText, string(CompletionLeaderPath))
		paths := c.pathCompleter(query)
		c.matches = make([]Match, len(paths))

		for i, p := range paths {
			c.matches[i] = Match{Value: p.Path, Label: p.Path}
		}
	case CompletionLeaderNone:
		return
	}

	if c.selected >= len(c.matches) {
		c.selected = max(0, len(c.matches)-1)
	}
}

// acceptText is the final token replacement (path drops @; command/skill keep leader + name).
func (c *CompletionState) acceptText(match Match) string {
	switch c.completionLeader { //nolint:exhaustive // None has no accept text
	case CompletionLeaderPath:
		return match.Value
	case CompletionLeaderCommand:
		return string(CompletionLeaderCommand) + match.Value
	case CompletionLeaderSkill:
		return string(CompletionLeaderSkill) + match.Value
	default:
		return match.Value
	}
}

// tabUserText keeps the leader while browsing (path keeps @; command/skill keep leader + value).
func (c *CompletionState) tabUserText(match Match) string {
	if c.CompletionLeader() == CompletionLeaderNone {
		return match.Value
	}

	return string(c.CompletionLeader()) + match.Value
}

// skillMatches renders skill names/descriptions as Matches.
func (c *CompletionState) skillMatches(options []*skills.Skill) []Match {
	out := make([]Match, 0, len(options))

	maxNameLen := 0
	for _, opt := range options {
		maxNameLen = max(maxNameLen, len(opt.Name))
	}

	maxDescLen := c.maxWidth - (maxNameLen + 2)

	for _, opt := range options {
		description := opt.Description
		if len(description) > maxDescLen {
			description = description[0:maxDescLen-3] + "..."
		}

		display := fmt.Sprintf("%-*s  %s", maxNameLen, opt.Name, description)
		out = append(out, Match{Value: opt.Name, Label: display})
	}

	return out
}

// commandMatches adapts Completer strings (often Usage lines) into Value=name, Label=usage.
func commandMatches(options []string) []Match {
	out := make([]Match, 0, len(options))

	for _, opt := range options {
		name := firstField(opt)
		if name == "" {
			continue
		}

		out = append(out, Match{Value: name, Label: opt})
	}

	return out
}

func firstField(s string) string {
	fields := strings.Fields(s)
	if len(fields) == 0 {
		return ""
	}

	return fields[0]
}

func atWordBoundary(currentText string) bool {
	if currentText == "" {
		return true
	}

	last, _ := utf8.DecodeLastRuneInString(currentText)

	return strings.ContainsRune(" \t\n", last)
}

// ReplaceActiveToken finds the active completion token in value and replaces it with replacement.
func ReplaceActiveToken(value, replacement string, leader byte) string {
	result, _ := replaceActiveToken(value, replacement, leader)
	return result
}

func replaceActiveToken(value, replacement string, leader byte) (string, int) {
	if value == "" {
		return replacement, len(replacement)
	}

	start := -1

	if leader == '/' {
		if value[0] == '/' {
			start = 0
		}
	} else {
		start = strings.LastIndexByte(value, leader)
	}

	if start < 0 {
		return value + replacement, len(value) + len(replacement)
	}

	end := start + 1
	for end < len(value) && value[end] != ' ' && value[end] != '\t' && value[end] != '\n' {
		end++
	}

	result := value[:start] + replacement + value[end:]

	return result, start + len(replacement)
}
