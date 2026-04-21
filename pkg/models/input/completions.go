package input

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/cneill/smoke/internal/uimsg"
)

type CompletionType int

const (
	CompletionTypeNone CompletionType = iota
	CompletionTypeCommand
	CompletionTypeSkill
)

type CompletionState struct {
	commandCompleter func(string) []string
	skillCompleter   func(string) []string
	completionType   CompletionType
	userText         string
	suggestedText    string
}

func NewCompletionState(commandCompleter, skillCompleter func(string) []string) (*CompletionState, error) {
	if commandCompleter == nil {
		return nil, fmt.Errorf("must supply command completer")
	}

	if skillCompleter == nil {
		return nil, fmt.Errorf("must supply skill completer")
	}

	return &CompletionState{
		commandCompleter: commandCompleter,
		skillCompleter:   skillCompleter,
	}, nil
}

func (c *CompletionState) InCommandCompletion() bool {
	return c.completionType == CompletionTypeCommand
}

func (c *CompletionState) InSkillCompletion() bool {
	return c.completionType == CompletionTypeSkill
}

func (c *CompletionState) InCompletion() bool {
	return c.InCommandCompletion() || c.InSkillCompletion()
}

// HandleUserCompletionKey returns true if 'msg' starts, or is part of, a completion for skills/commands.
func (c *CompletionState) HandleUserCompletionKey(msg tea.KeyMsg, currentText string) tea.Cmd {
	keyVal := msg.String()

	// check if we have a leading character signaling the start of a completion
	if !c.InCompletion() && !c.handleCompletionLeader(msg, currentText) {
		return nil
	}

	// TODO: handle tab/up(?) to fill in suggested text
	if msg.Type == tea.KeyBackspace {
		if c.userText != "" {
			c.userText = c.userText[:len(c.userText)-1]
		} else {
			c.Reset()
		}
	} else if msg.Type != tea.KeyTab {
		c.userText += keyVal
	}

	// user has cleared the whole completion text
	if c.userText == "" {
		c.Reset()
	}

	// TODO: sending this just for it to get converted to a statusline version seems dumb, figure out a reasonable fix
	// for the import cycle
	return uimsg.MsgToCmd(CompletionMessage{
		Text: c.CompletionText(),
	})
}

// CompletionText returns the full text that will be displayed in the autocompletion line
func (c *CompletionState) CompletionText() string {
	var options []string

	switch c.completionType {
	case CompletionTypeNone:
		return ""
	case CompletionTypeCommand:
		options = c.commandCompleter(strings.TrimPrefix(c.userText, "/"))
	case CompletionTypeSkill:
		options = c.skillCompleter(strings.TrimPrefix(c.userText, "$"))
	}

	if len(options) == 0 {
		c.suggestedText = ""
		return ""
	}

	strippedUserText := strings.TrimLeftFunc(c.userText, func(r rune) bool { return r == '/' || r == '$' })
	c.suggestedText = strings.TrimPrefix(options[0], strippedUserText)

	return c.userText + c.suggestedText
}

func (c *CompletionState) Reset() {
	c.userText = ""
	c.suggestedText = ""
	c.completionType = CompletionTypeNone
}

func (c *CompletionState) handleCompletionLeader(msg tea.KeyMsg, currentText string) bool {
	keyVal := msg.String()
	switch {
	case keyVal == "/" && currentText == "":
		c.completionType = CompletionTypeCommand
	case keyVal == "$":
		// make sure we're not in the middle of a word
		// TODO: handle unicode
		lastByte := string(currentText[len(currentText)-1])
		if currentText != "" && !strings.ContainsAny(lastByte, " \t\n") {
			return false
		}

		c.completionType = CompletionTypeSkill
	default:
		return false
	}

	return true
}
