// Package input contains a Bubbletea model to allow the user to enter 1) user messages for the LLM, and 2) prompt
// commands that may work with the session, exit the program, etc.
package input

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/cneill/smoke/internal/uimsg"
	"github.com/cneill/smoke/pkg/ask"
	"github.com/cneill/smoke/pkg/commands"
	"github.com/cneill/smoke/pkg/fs"
	"github.com/cneill/smoke/pkg/models/statusline"
	"github.com/mattn/go-runewidth"
)

const (
	black        = lipgloss.Color("#000000")
	orange       = lipgloss.Color("#cc4400")
	darkgray     = lipgloss.Color("#333333")
	MainSourceID = "MAIN" // TODO: this is a hack, but no other IDs exist right now; would change w/ e.g. tabs
)

type Opts struct {
	Width            int
	Height           int
	MaxHeight        int
	MaxContextWindow int64
	PlaceholderText  string
	CommandCompleter func(string) []string
	SkillCompleter   func(string) []string
	PathCompleter    func(string) []fs.PathMatch
}

func (o *Opts) OK() error {
	switch {
	case o.Width <= 0:
		return fmt.Errorf("width must be >0")
	case o.Height <= 0:
		return fmt.Errorf("height must be >0")
	case o.CommandCompleter == nil:
		return fmt.Errorf("must supply a command completer")
	case o.SkillCompleter == nil:
		return fmt.Errorf("must supply a skill completer")
	case o.PathCompleter == nil:
		return fmt.Errorf("must supply a path completer")
	}

	return nil
}

const (
	insertPrompt = "➜ "
	normalPrompt = "█ "
	askPrompt    = "? "
)

type mode int

const (
	modeInsert mode = iota
	modeNormal
)

type Model struct {
	statusline *statusline.Model
	textarea   textarea.Model
	spinner    spinner.Model

	waiting bool

	mode     mode
	vimState vimCommandState

	completionState *CompletionState
	askActive       bool

	// allocatedHeight is the last full input chrome height applied via Resize (textarea + path popup).
	// Used so layout deltas remain correct when the autocomplete popup appears or disappears.
	allocatedHeight int

	// Manages the full history of text submissions (LLM messages, prompt commands, etc) by the user for history
	// scrolling purposes *only*
	userHistory      []string
	userHistoryIndex *int
}

func New(opts *Opts) (*Model, error) {
	if err := opts.OK(); err != nil {
		return nil, fmt.Errorf("options error: %w", err)
	}

	cs, err := NewCompletionState(opts.CommandCompleter, opts.SkillCompleter, opts.PathCompleter)
	if err != nil {
		return nil, fmt.Errorf("error setting up completion state: %w", err)
	}

	model := &Model{
		statusline: statusline.New(opts.Width, opts.MaxContextWindow),
		textarea:   getTextArea(opts),
		spinner:    getSpinner(opts.Width, opts.Height),

		completionState: cs,
		allocatedHeight: opts.Height,

		mode: modeInsert,
	}

	return model, nil
}

func getTextArea(opts *Opts) textarea.Model {
	model := textarea.New()

	// TODO: make this fill the whole width with padding so it doesn't look awkward?
	// model.Placeholder = "Enter your message."
	// if opts.PlaceholderText != "" {
	// 	model.Placeholder = opts.PlaceholderText
	// }

	model.Focus()

	model.Prompt = insertPrompt
	model.CharLimit = 0
	model.Cursor.BlinkSpeed = time.Millisecond * 200

	model.SetWidth(opts.Width)
	model.SetHeight(opts.Height)

	model.MaxHeight = 5
	if opts.MaxHeight > 0 {
		model.MaxHeight = opts.MaxHeight
	}

	styleBase := lipgloss.NewStyle().
		Background(black)

	model.FocusedStyle.Base = styleBase
	model.FocusedStyle.CursorLine = styleBase.
		Foreground(lipgloss.Color("#eeeeee"))
	model.FocusedStyle.Placeholder = styleBase.
		Foreground(lipgloss.Color("#666666"))
	model.FocusedStyle.Text = styleBase.
		Foreground(lipgloss.Color("#eeeeee"))
	model.FocusedStyle.Prompt = styleBase.
		Foreground(orange).
		Bold(true)

	model.BlurredStyle.Base = styleBase
	model.BlurredStyle.CursorLine = model.FocusedStyle.CursorLine.
		Foreground(lipgloss.Color("#888888"))
	model.BlurredStyle.Placeholder = model.FocusedStyle.Placeholder.
		Foreground(lipgloss.Color("#444444"))
	model.BlurredStyle.Text = model.FocusedStyle.Text.
		Foreground(lipgloss.Color("#888888"))
	model.BlurredStyle.Prompt = model.FocusedStyle.Prompt.
		Foreground(darkgray)

	model.ShowLineNumbers = false

	return model
}

func getSpinner(width, height int) spinner.Model {
	model := spinner.New(
		spinner.WithSpinner(spinner.Points),
		spinner.WithStyle(
			lipgloss.NewStyle().
				Background(black).
				Foreground(lipgloss.Color("#ff0000")).
				Width(width).
				Height(height),
		),
	)

	return model
}

func (m *Model) Init() tea.Cmd {
	return tea.Batch(textarea.Blink, m.spinner.Tick)
}

func (m *Model) Update(msg tea.Msg) (*Model, tea.Cmd) {
	commands := []tea.Cmd{}
	startHeight := m.LayoutHeight()

	if cmd := m.handleSpinnerMsg(msg); cmd != nil {
		commands = append(commands, cmd)
	}

	if cmd := m.handleTextareaMsg(msg); cmd != nil {
		commands = append(commands, cmd)
	}

	if m.LayoutHeight() != startHeight {
		commands = append(commands, uimsg.MsgToCmd(ResizeMessage{}))
	}

	if cmd := m.handleStatuslineMsg(msg); cmd != nil {
		commands = append(commands, cmd)
	}

	return m, tea.Batch(commands...)
}

func (m *Model) View() string {
	mainContent := m.textarea.View()
	if m.waiting {
		mainContent = m.spinner.View()
	}

	if popup := m.completionPopupView(); popup != "" {
		return m.statusline.View() + "\n" + popup + "\n" + mainContent
	}

	return m.statusline.View() + "\n" + mainContent
}

func (m *Model) Resize(width, height int) {
	m.statusline.SetWidth(width)
	m.allocatedHeight = height

	// height is the full input chrome (textarea + optional popup); keep popup out of the textarea
	textHeight := max(1, height-m.PopupLines())
	m.textarea.SetWidth(width)
	m.textarea.SetHeight(textHeight)

	m.spinner.Style.Width(width)
	m.spinner.Style.Height(textHeight)
}

// LineHeight calculates the number of effective lines for the textarea only - both those ended with \n and those
// that are necessitated by text running off the screen - and returns the minimum of this or the maxlines of the
// textarea. Path popup height is excluded; use LayoutHeight for the full input chrome.
func (m *Model) LineHeight() int {
	content := m.textarea.Value()
	explicitLines := strings.Split(content, "\n")
	numLines := len(explicitLines)
	inputWidth := m.textarea.Width()

	for _, line := range explicitLines {
		lineWidth := runewidth.StringWidth(line)
		if lineWidth > inputWidth {
			extraLines := (lineWidth - 1) / inputWidth
			numLines += extraLines
		}
	}

	result := min(m.textarea.MaxHeight, numLines+1) // padding

	return result
}

// LayoutHeight is the full vertical size of the input view (textarea + optional path popup).
// Statusline is accounted for separately by the parent (+1).
func (m *Model) LayoutHeight() int {
	return m.LineHeight() + m.PopupLines()
}

func (m *Model) GetWidth() int {
	return m.textarea.Width()
}

func (m *Model) GetHeight() int {
	if m.allocatedHeight > 0 {
		return m.allocatedHeight
	}

	return m.textarea.Height()
}

func (m *Model) Focused() bool {
	return m.textarea.Focused()
}

func (m *Model) Waiting() bool { return m.waiting }

func (m *Model) BeginAsk() {
	m.askActive = true
	m.setInputMode(modeInsert)
	m.textarea.Focus()
	m.statusline.SetFocus(true)
	m.textarea.Prompt = askPrompt
	m.completionState.Reset()
}

func (m *Model) ClearAsk() {
	m.askActive = false
	m.setInputMode(modeInsert)
}

func (m *Model) SetWaiting(value bool) tea.Cmd {
	m.waiting = value
	if value {
		return m.spinner.Tick
	}

	return nil
}

// PopupLines returns how many terminal lines the completion popup currently occupies (0 if hidden).
func (m *Model) PopupLines() int {
	return m.completionState.PopupLineCount()
}

func (m *Model) handleTextareaMsg(msg tea.Msg) tea.Cmd { //nolint:cyclop,gocognit,funlen
	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok && !m.waiting {
		newTextarea, cmd := m.textarea.Update(msg)
		m.textarea = newTextarea

		return cmd
	}

	// catch an escape key when waiting to abort request; if not escape, continue
	if m.waiting {
		if cmd := m.handleWaitingKey(keyMsg); cmd != nil {
			return cmd
		}
	}

	// Autocompletion intercepts keys before enter/esc/history handling when active or starting.
	if m.Focused() && m.mode == modeInsert && !m.waiting {
		result := m.completionState.HandleKey(keyMsg, m.textarea.Value())
		if result.Replace != "" || result.Consume {
			return m.applyKeyResult(result)
		}
	}

	switch keyMsg.Type { //nolint:exhaustive
	case tea.KeyEnter:
		// handle a user message to the assistant or a prompt command
		if m.Focused() && m.mode == modeInsert {
			return m.handleContentSubmit()
		}
	case tea.KeyEsc:
		// check if the user is currently in the process of answering a question from the ask tool
		// TODO: figure out if this should override VIM/scroll switching - may be annoying
		if m.askActive && m.Focused() && m.mode == modeInsert {
			m.textarea.Reset()
			m.ClearAsk()

			return uimsg.MsgToCmd(ask.UserCanceledMessage{})
		}

		if !m.Focused() {
			return nil
		}

		// insert -> normal mode
		if m.mode == modeInsert {
			m.setInputMode(modeNormal)
			return nil
		}

		if m.vimState.active() {
			m.vimState.reset()
			return nil
		}

		// modeNormal -> history (blur)
		m.textarea.Blur()
		m.statusline.SetFocus(false)

		return nil

	case tea.KeyShiftTab:
		return uimsg.MsgToCmd(ShiftModeMessage{})

	case tea.KeyUp, tea.KeyDown:
		// Let the textarea move within a multiline history entry. History is
		// traversed only when moving beyond the entry's first or last line.
		if m.handleHistoryTraversal(keyMsg) {
			return nil
		}

	case tea.KeyRunes:
		// History mode: allow i/A/I/o/O to re-enter insert mode
		if !m.Focused() {
			return m.handleVimInsertKey(keyMsg.String())
		}

		if m.mode == modeNormal {
			return m.handleNormalModeVimKey(keyMsg.String())
		}
	}

	// don't send key updates to the textarea when scrolling the history viewport
	if m.Focused() && !m.waiting {
		newTextarea, cmd := m.textarea.Update(keyMsg)
		m.textarea = newTextarea

		return cmd
	}

	return nil
}

func (m *Model) setInputMode(newMode mode) {
	m.mode = newMode
	if m.mode == modeInsert {
		m.textarea.Prompt = insertPrompt
	} else {
		m.textarea.Prompt = normalPrompt
	}
}

func (m *Model) handleSpinnerMsg(msg tea.Msg) tea.Cmd {
	if !m.waiting {
		return nil
	}

	newSpinner, cmd := m.spinner.Update(msg)
	m.spinner = newSpinner

	return cmd
}

func (m *Model) handleWaitingKey(msg tea.KeyMsg) tea.Cmd {
	if msg.Type == tea.KeyEsc {
		m.waiting = false

		return uimsg.MsgToCmd(CancelUserMessage{
			SourceID: MainSourceID,
			Err:      fmt.Errorf("user aborted request"),
		})
	}

	return nil
}

func (m *Model) handleHistoryTraversal(msg tea.KeyMsg) bool { //nolint:cyclop
	// History is available only from a focused insert-mode textarea.
	if !m.Focused() || m.mode != modeInsert || m.waiting || len(m.userHistory) == 0 {
		return false
	}

	// Do not steal arrows from ordinary editing. An active history index means
	// the current value came from history and is eligible for traversal.
	if m.textarea.Value() != "" && m.userHistoryIndex == nil {
		return false
	}

	if m.userHistoryIndex != nil {
		line := m.textarea.Line()
		lineInfo := m.textarea.LineInfo()
		lastLine := m.textarea.LineCount() - 1
		atFirstRenderedRow := line == 0 && lineInfo.RowOffset == 0
		atLastRenderedRow := line == lastLine && lineInfo.RowOffset == lineInfo.Height-1

		switch msg.Type { //nolint:exhaustive
		case tea.KeyUp:
			if !atFirstRenderedRow {
				return false
			}
		case tea.KeyDown:
			if !atLastRenderedRow {
				return false
			}
		}
	}

	switch msg.Type { //nolint:exhaustive
	case tea.KeyUp:
		if m.userHistoryIndex != nil {
			if *m.userHistoryIndex == 0 {
				return true
			}

			*m.userHistoryIndex--
		} else {
			idx := len(m.userHistory) - 1
			m.userHistoryIndex = &idx
		}
	case tea.KeyDown:
		if m.userHistoryIndex == nil {
			return false
		}

		if *m.userHistoryIndex < len(m.userHistory)-1 {
			*m.userHistoryIndex++
		} else {
			m.userHistoryIndex = nil
			m.textarea.SetValue("")

			return true
		}
	default:
		return false
	}

	m.textarea.SetValue(m.userHistory[*m.userHistoryIndex])

	if msg.Type == tea.KeyUp {
		setLastRenderedRowStart(&m.textarea)
	} else {
		setLogicalCursor(&m.textarea, logicalPosition{})
	}

	return true
}

// handleContentSubmit interprets the content the user has entered in the textarea and returns an appropriate tea.Cmd.
func (m *Model) handleContentSubmit() tea.Cmd {
	content := m.textarea.Value()
	m.textarea.Reset()
	m.userHistory = append(m.userHistory, content)
	m.userHistoryIndex = nil
	m.completionState.Reset()

	switch {
	// user is answering a question
	case m.askActive:
		return uimsg.MsgToCmd(ask.UserInputMessage{Content: content})
	// user has sent a prompt command like "/help"
	case strings.HasPrefix(content, "/"):
		return m.handlePromptCommand(content)
	// user has sent a normal message of some kind
	default:
		return uimsg.MsgToCmd(UserMessage{
			SourceID: MainSourceID,
			Content:  content,
		})
	}
}

// handlePromptCommand checks for a command specified by the user (e.g. "/exit") and returns the appropriate message
// struct with the arguments parsed and populated.
func (m *Model) handlePromptCommand(content string) tea.Cmd {
	fields := strings.Fields(content)
	cmdName := strings.TrimPrefix(fields[0], "/")

	m.completionState.Reset()

	args := []string{}
	if len(fields) > 1 {
		args = fields[1:]
	}

	promptMsg := commands.PromptMessage{
		Command: cmdName,
		Args:    args,
	}

	return uimsg.MsgToCmd(promptMsg)
}

func (m *Model) handleStatuslineMsg(msg tea.Msg) tea.Cmd {
	newStatusline, cmd := m.statusline.Update(msg)
	m.statusline = newStatusline

	return cmd
}

func (m *Model) applyKeyResult(result KeyResult) tea.Cmd {
	if result.Replace != "" {
		leader := result.Leader
		if leader == 0 {
			leader = m.completionState.CompletionLeader()
		}

		m.replaceActiveToken(result.Replace, byte(leader))
	}

	return nil
}

// replaceActiveToken finds the active completion token in the textarea and replaces it with replacement.
func (m *Model) replaceActiveToken(replacement string, leader byte) {
	value := m.textarea.Value()
	newValue, cursor := replaceActiveToken(value, replacement, leader)
	m.textarea.SetValue(newValue)
	m.textarea.SetCursor(cursor)
}

func (m *Model) completionPopupView() string {
	if !m.completionState.PopupActive() {
		return ""
	}

	matches := m.completionState.Matches()
	selected := m.completionState.Selected()
	start, end := m.completionState.VisibleRange()

	var (
		panelBG    = lipgloss.Color("#11161d")
		muted      = lipgloss.Color("#7d8796")
		orangeCol  = orange
		textCol    = lipgloss.Color("#d7e0ea")
		selectedBG = lipgloss.Color("#321b05")
		white      = lipgloss.Color("#ffffff")
	)

	itemStyle := lipgloss.NewStyle().Foreground(textCol).Background(panelBG)
	selectedStyle := lipgloss.NewStyle().Foreground(white).Background(selectedBG).Bold(true)
	cursorStyle := lipgloss.NewStyle().Foreground(orangeCol).Background(selectedBG).Bold(true)
	helpStyle := lipgloss.NewStyle().Foreground(muted).Background(panelBG)
	container := lipgloss.NewStyle().
		Background(panelBG).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#3f4856")).
		Padding(0, 1).
		Width(max(20, m.textarea.Width()-2))

	var sb strings.Builder

	for idx := start; idx < end; idx++ {
		label := matches[idx].Label
		if label == "" {
			label = matches[idx].Value
		}

		if idx == selected {
			fmt.Fprintln(&sb, cursorStyle.Render("➜ ")+selectedStyle.Render(label))
		} else {
			fmt.Fprintln(&sb, itemStyle.Render("  "+label))
		}
	}

	if end-start < len(matches) {
		fmt.Fprintln(&sb, helpStyle.Render(fmt.Sprintf("%d-%d of %d  tab: fill • enter: accept • esc: dismiss", start+1, end, len(matches))))
	}

	return container.Render(strings.TrimRight(sb.String(), "\n"))
}
