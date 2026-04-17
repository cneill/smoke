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
	"github.com/cneill/smoke/pkg/commands"
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
	PlaceholderText  string
	CommandCompleter func(string) []string
	SkillCompleter   func(string) []string
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
	}

	return nil
}

const (
	insertPrompt = "▶ "
	normalPrompt = "█ "
)

type mode int

const (
	modeInsert mode = iota
	modeNormal
)

type Model struct {
	textarea textarea.Model
	spinner  spinner.Model

	waiting bool

	mode     mode
	pendingD bool
	lastD    time.Time

	completionState *CompletionState

	topLineBorderFocused     lipgloss.Style
	topLineBorderBlurred     lipgloss.Style
	topLineSuggestionFocused lipgloss.Style
	topLineSuggestionBlurred lipgloss.Style
	topLineUsageFocused      lipgloss.Style
	topLineUsageBlurred      lipgloss.Style
	inputTokens              int64
	outputTokens             int64

	// Manages the full history of text submissions (LLM messages, prompt commands, etc) by the user for history
	// scrolling purposes *only*
	userHistory      []string
	userHistoryIndex *int
}

func New(opts *Opts) (*Model, error) {
	if err := opts.OK(); err != nil {
		return nil, fmt.Errorf("options error: %w", err)
	}

	cs, err := NewCompletionState(opts.CommandCompleter, opts.SkillCompleter)
	if err != nil {
		return nil, fmt.Errorf("error setting up completion state: %w", err)
	}

	model := &Model{
		textarea: getTextArea(opts),
		spinner:  getSpinner(opts.Width, opts.Height),

		completionState: cs,

		mode: modeInsert,

		topLineBorderFocused: lipgloss.NewStyle().
			Foreground(orange).
			Background(black).
			Align(lipgloss.Left),
		topLineBorderBlurred: lipgloss.NewStyle().
			Foreground(darkgray).
			Background(black).
			Bold(true).
			Align(lipgloss.Left),
		topLineSuggestionFocused: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#ffffff")).
			Background(orange).
			Bold(true).
			Align(lipgloss.Left),
		topLineSuggestionBlurred: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#ffffff")).
			Background(lipgloss.Color("#aaaaaa")).
			Align(lipgloss.Left),
		topLineUsageFocused: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#00ff00")).
			Background(lipgloss.Color("#111111")).
			Bold(true).
			Align(lipgloss.Left),
		topLineUsageBlurred: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#cccccc")).
			Background(black).
			Align(lipgloss.Left),
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

	model.FocusedStyle.Base = lipgloss.NewStyle().
		Background(lipgloss.Color("#000000"))
	model.FocusedStyle.CursorLine = lipgloss.NewStyle().
		Background(lipgloss.Color("#000000")).
		Foreground(lipgloss.Color("#eeeeee"))
	model.FocusedStyle.Placeholder = lipgloss.NewStyle().
		Background(lipgloss.Color("#000000")).
		Foreground(lipgloss.Color("#666666"))
	model.FocusedStyle.Text = lipgloss.NewStyle().
		Background(lipgloss.Color("#000000")).
		Foreground(lipgloss.Color("#eeeeee"))
	model.FocusedStyle.Prompt = lipgloss.NewStyle().
		Background(lipgloss.Color("#000000")).
		Foreground(orange).
		Bold(true)

	model.BlurredStyle.Base = lipgloss.NewStyle().
		Inherit(model.FocusedStyle.Base)
	model.BlurredStyle.CursorLine = lipgloss.NewStyle().
		Inherit(model.FocusedStyle.CursorLine).
		Foreground(lipgloss.Color("#888888"))
	model.BlurredStyle.Placeholder = lipgloss.NewStyle().
		Inherit(model.FocusedStyle.Placeholder).
		Foreground(lipgloss.Color("#444444"))
	model.BlurredStyle.Text = lipgloss.NewStyle().
		Inherit(model.FocusedStyle.Text).
		Foreground(lipgloss.Color("#888888"))
	model.BlurredStyle.Prompt = lipgloss.NewStyle().
		Inherit(model.FocusedStyle.Prompt).
		Foreground(darkgray)

	model.ShowLineNumbers = false

	return model
}

// TODO: clean up the duplication between textarea and spinner styling...
func getSpinner(width, height int) spinner.Model {
	model := spinner.New(
		spinner.WithSpinner(spinner.Points),
		spinner.WithStyle(
			lipgloss.NewStyle().
				Background(lipgloss.Color("#000000")).
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
	startHeight := m.textarea.Height()

	if cmd := m.handleSpinnerMsg(msg); cmd != nil {
		commands = append(commands, cmd)
	}

	if cmd := m.handleTextareaMsg(msg); cmd != nil {
		commands = append(commands, cmd)
	}

	if m.LineHeight() != startHeight {
		commands = append(commands, func() tea.Msg {
			return ResizeMessage{}
		})
	}

	return m, tea.Batch(commands...)
}

func (m *Model) View() string {
	mainContent := m.textarea.View()
	if m.waiting {
		mainContent = m.spinner.View()
	}

	return m.topLineView() + "\n" + mainContent
}

func (m *Model) Resize(width, height int) {
	m.textarea.SetWidth(width)
	m.textarea.SetHeight(height)
	m.spinner.Style.Width(width)
	m.spinner.Style.Height(height)
}

// LineHeight calculates the number of effective lines - both those ended with \n and those that are necessitated by
// text running off the screen - and returns the minimum of this or the maxlines of the textarea.
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

func (m *Model) GetWidth() int {
	return m.textarea.Width()
}

func (m *Model) GetHeight() int {
	return m.textarea.Height()
}

func (m *Model) Focused() bool {
	return m.textarea.Focused()
}

func (m *Model) Waiting() bool { return m.waiting }

func (m *Model) SetWaiting(value bool) tea.Cmd {
	m.waiting = value
	if value {
		return m.spinner.Tick
	}

	return nil
}

func (m *Model) UpdateUsage(inputTokens, outputTokens int64) {
	m.inputTokens = inputTokens
	m.outputTokens = outputTokens
}

func (m *Model) topLineView() string {
	var (
		borderStyle     = m.topLineBorderFocused
		suggestionStyle = m.topLineSuggestionFocused
		usageStyle      = m.topLineUsageFocused
	)
	if !m.Focused() {
		borderStyle = m.topLineBorderBlurred
		suggestionStyle = m.topLineSuggestionBlurred
		usageStyle = m.topLineUsageBlurred
	}

	totalWidth := m.textarea.Width()
	usageText := usageStyle.Render(fmt.Sprintf("in: %d, out: %d", m.inputTokens, m.outputTokens))
	usagePadding := borderStyle.Render("█")
	usageWidth := lipgloss.Width(usageText)
	border := borderStyle.Render(strings.Repeat("▄", totalWidth-usageWidth) + "█")

	// add the prompt command completion suggestions above the "/" in the topline
	if completion := m.completionState.CompletionText(); completion != "" {
		suggestionWidth := lipgloss.Width(completion)
		border = borderStyle.Render(strings.Repeat("▄", 2)) +
			suggestionStyle.Render(completion) +
			borderStyle.Render(strings.Repeat("▄", totalWidth-usageWidth-suggestionWidth-2)+"█")
	}

	return border + usageText + usagePadding
}

func (m *Model) handleTextareaMsg(msg tea.Msg) tea.Cmd {
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

	switch keyMsg.Type { //nolint:exhaustive
	case tea.KeyEnter:
		// handle a user message to the assistant or a prompt command
		if m.Focused() && m.mode == modeInsert {
			return m.handleContentSubmit()
		}
	case tea.KeyEsc:
		if !m.Focused() {
			return nil
		}

		// insert -> normal mode
		if m.mode == modeInsert {
			m.setMode(modeNormal)
			return nil
		}

		// modeNormal -> history (blur)
		m.textarea.Blur()

		return nil

	case tea.KeyUp, tea.KeyDown:
		// scroll up and down through previous user messages
		if cmd := m.handleHistoryTraversal(keyMsg); cmd != nil {
			return cmd
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

	if m.completionState.HandleUserCompletionKey(keyMsg, m.textarea.Value()) {
		var cmd tea.Cmd

		m.textarea, cmd = m.textarea.Update(msg)

		return cmd
	}

	// don't send key updates to the textarea when scrolling the history viewport
	if m.Focused() && !m.waiting {
		newTextarea, cmd := m.textarea.Update(keyMsg)
		m.textarea = newTextarea

		return cmd
	}

	return nil
}

func (m *Model) setMode(newMode mode) {
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

func (m *Model) handleHistoryTraversal(msg tea.KeyMsg) tea.Cmd { //nolint:cyclop // This used to be much worse...
	// We only want to scroll history if we're focused, in insert mode, and not waiting
	if !m.Focused() || m.mode != modeInsert || m.waiting {
		return nil
	}

	// We only want to traverse history if 1) there is no text in the textarea, or 2) we're already traversing it
	// TODO: do a partial match on what the user has already entered to filter history messages?
	if m.textarea.Value() != "" && m.userHistoryIndex == nil {
		return nil
	}

	switch msg.Type { //nolint:exhaustive
	case tea.KeyUp:
		if m.userHistoryIndex != nil && *m.userHistoryIndex > 0 {
			*m.userHistoryIndex--
		} else if m.userHistoryIndex == nil && len(m.userHistory) > 0 {
			idx := len(m.userHistory) - 1
			m.userHistoryIndex = &idx
		}
	case tea.KeyDown:
		if m.userHistoryIndex != nil {
			if *m.userHistoryIndex < len(m.userHistory)-1 {
				*m.userHistoryIndex++
			} else {
				m.userHistoryIndex = nil
				m.textarea.SetValue("")
			}
		}
	}

	if m.userHistoryIndex != nil {
		m.textarea.SetValue(m.userHistory[*m.userHistoryIndex])
	}

	return nil
}

// handleContentSubmit interprets the content the user has entered in the textarea and returns an appropriate tea.Cmd.
func (m *Model) handleContentSubmit() tea.Cmd {
	content := m.textarea.Value()
	m.textarea.Reset()
	m.userHistory = append(m.userHistory, content)
	m.userHistoryIndex = nil

	if strings.HasPrefix(content, "/") {
		return m.handlePromptCommand(content)
	}

	return uimsg.MsgToCmd(UserMessage{
		SourceID: MainSourceID,
		Content:  content,
	})
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

	return uimsg.MsgToCmd(commands.PromptMessage{
		Command: cmdName,
		Args:    args,
	})
}
