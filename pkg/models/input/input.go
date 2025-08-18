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
	"github.com/cneill/smoke/pkg/commands"
	"github.com/mattn/go-runewidth"
)

type Opts struct {
	Width            int
	Height           int
	MaxHeight        int
	PlaceholderText  string
	CommandCompleter func(string) []string
}

func (o *Opts) OK() error {
	switch {
	case o.Width <= 0:
		return fmt.Errorf("width must be >0")
	case o.Height <= 0:
		return fmt.Errorf("height must be >0")
	case o.CommandCompleter == nil:
		return fmt.Errorf("must supply a command completer")
	}

	return nil
}

// const prompt = "│"
const (
	insertPrompt = "▶ "
	normalPrompt = "▷ "
)

type mode int

const (
	modeInsert mode = iota
	modeNormal
)

type Model struct {
	textarea textarea.Model
	spinner  spinner.Model

	commandCompleter func(string) []string

	waiting bool

	mode     mode
	pendingG bool
	lastG    time.Time
	// inCommandCompletion     bool
	// userCompletionText      string
	// suggestedCompletionText string
}

func New(opts *Opts) (*Model, error) {
	if err := opts.OK(); err != nil {
		return nil, fmt.Errorf("options error: %w", err)
	}

	model := &Model{
		textarea: getTextArea(opts),
		spinner:  getSpinner(opts.Width, opts.Height),

		commandCompleter: opts.CommandCompleter,

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

	model.SetWidth(opts.Width)
	model.SetHeight(opts.Height)

	model.MaxHeight = 5
	if opts.MaxHeight > 0 {
		model.MaxHeight = opts.MaxHeight
	}

	orange := lipgloss.Color("#cc4400")

	model.FocusedStyle.Base = lipgloss.NewStyle().
		BorderStyle(lipgloss.OuterHalfBlockBorder()).
		BorderTopForeground(orange).
		BorderTopBackground(lipgloss.Color("#000000")).
		BorderTop(true).
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
		Inherit(model.FocusedStyle.Base).
		BorderTopForeground(lipgloss.Color("#333333"))
	model.BlurredStyle.CursorLine = lipgloss.NewStyle().
		Inherit(model.FocusedStyle.CursorLine).
		Foreground(lipgloss.Color("#888888"))
	model.BlurredStyle.Placeholder = lipgloss.NewStyle().
		Inherit(model.FocusedStyle.Placeholder).
		Foreground(lipgloss.Color("#444444"))
	model.BlurredStyle.Text = lipgloss.NewStyle().
		Inherit(model.FocusedStyle.Text).
		Foreground(lipgloss.Color("#8888888"))
	model.BlurredStyle.Prompt = lipgloss.NewStyle().
		Inherit(model.FocusedStyle.Prompt).
		Foreground(lipgloss.Color("#888888"))

	model.ShowLineNumbers = false

	return model
}

func getSpinner(width, height int) spinner.Model {
	orange := lipgloss.Color("#cc4400")

	model := spinner.New(
		spinner.WithSpinner(spinner.Points),
		spinner.WithStyle(
			lipgloss.NewStyle().
				BorderStyle(lipgloss.OuterHalfBlockBorder()).
				BorderTopForeground(orange).
				BorderTopBackground(lipgloss.Color("#000000")).
				BorderTop(true).
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
	if m.waiting {
		return m.spinner.View()
	}

	// if m.userCompletionText != "" {
	// 	return m.userCompletionText + m.suggestedCompletionText
	// }

	return m.textarea.View()
}

func (m *Model) Resize(width, height int) {
	m.textarea.SetWidth(width)
	m.textarea.SetHeight(height)
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

func (m *Model) handleTextareaMsg(msg tea.Msg) tea.Cmd {
	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok && !m.waiting {
		newTextarea, cmd := m.textarea.Update(msg)
		m.textarea = newTextarea

		return cmd
	}

	if m.waiting {
		if keyMsg.Type == tea.KeyEsc {
			m.waiting = false

			return wrapMsg(CancelUserMessage{
				Err: fmt.Errorf("user aborted request"),
			})
		}

		return nil
	}

	switch keyMsg.Type { //nolint:exhaustive
	case tea.KeyEnter:
		if m.Focused() && m.mode == modeInsert {
			return m.handleContentSubmit()
		}
	case tea.KeyEsc:
		if !m.Focused() {
			return nil
		}

		if m.mode == modeInsert {
			m.setMode(modeNormal)
			return nil
		}
		// modeNormal -> history (blur)
		m.textarea.Blur()

		return nil
	case tea.KeyRunes:
		// History mode: allow i/A/I/o/O to re-enter insert mode
		if !m.Focused() {
			return m.handleVimKeyBindingsHistory(keyMsg.String())
		}

		if m.mode == modeNormal {
			return m.handleNormalModeRunes(keyMsg.String())
		}

		// if (msg.String() == "/" && m.textarea.Value() == "") || m.userCompletionText != "" {
		//      return m.handleCommandCompletion(msg)
		// }

		// modeInsert will fall through to textarea for insertion
	}

	newTextarea, cmd := m.textarea.Update(keyMsg)
	m.textarea = newTextarea

	return cmd
}

func (m *Model) handleVimKeyBindingsHistory(key string) tea.Cmd {
	switch key {
	case "i":
		m.setMode(modeInsert)
		m.textarea.Focus()

		return textarea.Blink
	case "a":
		m.setMode(modeInsert)
		m.textarea.Focus()
		m.textarea, _ = m.textarea.Update(tea.KeyMsg{Type: tea.KeyRight})

		return textarea.Blink
	case "A":
		m.setMode(modeInsert)
		m.textarea.Focus()
		m.textarea.CursorEnd()

		return textarea.Blink
	case "I":
		m.setMode(modeInsert)
		m.textarea.Focus()
		m.textarea.CursorStart()

		return textarea.Blink
	case "o":
		m.setMode(modeInsert)
		m.textarea.Focus()
		m.textarea.CursorEnd()
		m.textarea.InsertString("\n")

		return textarea.Blink
	case "O":
		m.setMode(modeInsert)
		m.textarea.Focus()
		m.textarea.CursorStart()
		m.textarea.InsertString("\n")
		m.textarea.CursorUp()

		return textarea.Blink
	}

	return nil
}

func (m *Model) handleNormalModeRunes(key string) tea.Cmd {
	switch key {
	case "h":
		m.textarea, _ = m.textarea.Update(tea.KeyMsg{Type: tea.KeyLeft})
		return nil
	case "l":
		m.textarea, _ = m.textarea.Update(tea.KeyMsg{Type: tea.KeyRight})
		return nil
	case "j":
		m.textarea, _ = m.textarea.Update(tea.KeyMsg{Type: tea.KeyDown})
		return nil
	case "k":
		m.textarea, _ = m.textarea.Update(tea.KeyMsg{Type: tea.KeyUp})
		return nil
	case "0":
		m.textarea.CursorStart()
		return nil
	case "$":
		m.textarea.CursorEnd()
		return nil
	case "w":
		// Move to beginning of next word
		content := m.textarea.Value()
		pos := m.textarea.LineInfo().ColumnOffset
		newPos := findNextWord(content, pos)
		m.textarea.SetCursor(newPos)

		return nil
	case "W":
		// Move to beginning of next WORD
		content := m.textarea.Value()
		pos := m.textarea.LineInfo().ColumnOffset
		newPos := findNextWORD(content, pos)
		m.textarea.SetCursor(newPos)

		return nil
	case "e":
		// Move to end of current/next word
		content := m.textarea.Value()
		pos := m.textarea.LineInfo().ColumnOffset
		newPos := findEndOfWord(content, pos)
		m.textarea.SetCursor(newPos)

		return nil
	case "E":
		// Move to end of current/next WORD
		content := m.textarea.Value()
		pos := m.textarea.LineInfo().ColumnOffset
		newPos := findEndOfWORD(content, pos)
		m.textarea.SetCursor(newPos)

		return nil
	case "b":
		// Move backward to beginning of word
		content := m.textarea.Value()
		pos := m.textarea.LineInfo().ColumnOffset
		newPos := findPrevWord(content, pos)
		m.textarea.SetCursor(newPos)

		return nil
	case "B":
		// Move backward to beginning of WORD
		content := m.textarea.Value()
		pos := m.textarea.LineInfo().ColumnOffset
		newPos := findPrevWORD(content, pos)
		m.textarea.SetCursor(newPos)

		return nil
	case "g":
		if m.pendingG && time.Since(m.lastG) <= time.Second {
			m.textarea.CursorStart()
			m.pendingG = false

			return nil
		}

		m.pendingG = true
		m.lastG = time.Now()

		return nil
	case "G":
		m.textarea.CursorEnd()
		m.pendingG = false

		return nil
	case "i":
		m.setMode(modeInsert)

		return textarea.Blink
	case "a":
		m.textarea, _ = m.textarea.Update(tea.KeyMsg{Type: tea.KeyRight})
		m.setMode(modeInsert)

		return textarea.Blink
	case "A":
		m.textarea.CursorEnd()
		m.setMode(modeInsert)

		return textarea.Blink
	case "I":
		m.textarea.CursorStart()
		m.setMode(modeInsert)

		return textarea.Blink
	case "o":
		m.textarea.CursorEnd()
		m.textarea.InsertString("\n")
		m.setMode(modeInsert)

		return textarea.Blink
	case "O":
		m.textarea.CursorStart()
		m.textarea.InsertString("\n")
		m.textarea.CursorUp()
		m.setMode(modeInsert)

		return textarea.Blink
	case "p":
		return textarea.Paste
	}

	// Unrecognized in normal mode: do nothing
	m.pendingG = false

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

// func (m *Model) handleCommandCompletion(msg tea.KeyMsg) tea.Cmd {
// 	if msg.Type == tea.KeyBackspace {
// 		m.userCompletionText = m.userCompletionText[:len(m.userCompletionText)-1]
// 	} else {
// 		m.userCompletionText += msg.String()
// 	}
//
// 	cmdPart := strings.TrimPrefix(m.userCompletionText, "/")
// 	options := m.commandCompleter(cmdPart)
//
// 	if len(options) == 0 {
// 		return nil
// 	}
//
// 	m.suggestedCompletionText = strings.TrimPrefix(options[0], cmdPart)
//
// 	var cmd tea.Cmd
// 	m.textarea, cmd = m.textarea.Update(msg)
//
// 	m.textarea.SetValue(m.userCompletionText + m.suggestedCompletionText)
// 	m.textarea.SetCursor(len(m.userCompletionText))
//
// 	slog.Debug("handling command completion", "user", m.userCompletionText, "suggested", m.suggestedCompletionText)
//
// 	return cmd
// }

func (m *Model) handleSpinnerMsg(msg tea.Msg) tea.Cmd {
	if !m.waiting {
		return nil
	}

	newSpinner, cmd := m.spinner.Update(msg)
	m.spinner = newSpinner

	return cmd
}

// handleContentSubmit interprets the content the user has entered in the textarea and returns an appropriate tea.Cmd.
func (m *Model) handleContentSubmit() tea.Cmd {
	content := m.textarea.Value()
	m.textarea.Reset()

	if strings.HasPrefix(content, "/") {
		return m.handlePromptCommand(content)
	}

	return wrapMsg(UserMessage{
		Content: content,
	})
}

// handlePromptCommand checks for a command specified by the user (e.g. "/exit") and returns the appropriate message
// struct with the arguments parsed and populated.
func (m *Model) handlePromptCommand(content string) tea.Cmd {
	fields := strings.Fields(content)
	cmdName := strings.TrimPrefix(fields[0], "/")

	// m.userCompletionText = ""
	// m.suggestedCompletionText = ""

	args := []string{}
	if len(fields) > 1 {
		args = fields[1:]
	}

	return wrapMsg(commands.PromptCommandMessage{
		Command: cmdName,
		Args:    args,
	})
}

func wrapMsg(msg tea.Msg) tea.Cmd {
	return func() tea.Msg {
		return msg
	}
}
