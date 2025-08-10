package input

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-runewidth"
)

type Opts struct {
	Width           int
	Height          int
	MaxHeight       int
	PlaceholderText string
}

func (o *Opts) OK() error {
	switch {
	case o.Width <= 0:
		return fmt.Errorf("width must be >0")
	case o.Height <= 0:
		return fmt.Errorf("height must be >0")
	}

	return nil
}

const prompt = "│"

type Model struct {
	textarea textarea.Model
	spinner  spinner.Model

	waiting bool
}

func New(opts *Opts) (*Model, error) {
	if err := opts.OK(); err != nil {
		return nil, fmt.Errorf("options error: %w", err)
	}

	model := &Model{
		textarea: getTextArea(opts),
		spinner:  getSpinner(opts.Height),
	}

	return model, nil
}

func getTextArea(opts *Opts) textarea.Model {
	model := textarea.New()

	model.Placeholder = "Enter your message."
	if opts.PlaceholderText != "" {
		model.Placeholder = opts.PlaceholderText
	}

	model.Focus()

	model.Prompt = prompt
	model.CharLimit = 0

	model.SetWidth(opts.Width)
	model.SetHeight(opts.Height)

	model.MaxHeight = 5
	if opts.MaxHeight > 0 {
		model.MaxHeight = opts.MaxHeight
	}

	// Focused
	model.FocusedStyle.CursorLine = lipgloss.NewStyle().
		Background(lipgloss.Color("#000000")).
		Foreground(lipgloss.Color("#eeeeee"))
	model.FocusedStyle.Text = lipgloss.NewStyle().
		Background(lipgloss.Color("#000000")).
		Foreground(lipgloss.Color("#eeeeee"))
	model.FocusedStyle.Prompt = lipgloss.NewStyle().
		Bold(true).
		Padding(2, 2, 2, 2)

	// Blurred
	model.BlurredStyle.CursorLine = lipgloss.NewStyle().
		Background(lipgloss.Color("#000000")).
		Foreground(lipgloss.Color("#888888"))
	model.BlurredStyle.Text = lipgloss.NewStyle().
		Background(lipgloss.Color("#000000")).
		Foreground(lipgloss.Color("#888888"))
	model.BlurredStyle.Prompt = lipgloss.NewStyle().
		Bold(true).
		Padding(2, 2, 2, 2)

	model.ShowLineNumbers = false
	// model.KeyMap.InsertNewline.SetEnabled(false)

	return model
}

func getSpinner(height int) spinner.Model {
	model := spinner.New(
		spinner.WithSpinner(spinner.Ellipsis),
		spinner.WithStyle(
			lipgloss.NewStyle().
				Foreground(lipgloss.Color("#ff0000")).
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

func (m *Model) handleTextareaMsg(msg tea.Msg) tea.Cmd {
	if m.waiting {
		return nil
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type { //nolint:exhaustive
		case tea.KeyEnter:
			if m.Focused() {
				return m.handleContentSubmit()
			}
		case tea.KeyEsc:
			m.textarea.Blur()
		case tea.KeyRunes:
			if !m.Focused() && msg.String() == "i" {
				m.textarea.Focus()
				return nil
			}
		}
	}

	newTextarea, cmd := m.textarea.Update(msg)
	m.textarea = newTextarea

	return cmd
}

func (m *Model) handleSpinnerMsg(msg tea.Msg) tea.Cmd {
	if !m.waiting {
		return nil
	}

	newSpinner, cmd := m.spinner.Update(msg)
	m.spinner = newSpinner

	return cmd
}

func (m *Model) View() string {
	if m.waiting {
		return "Waiting" + m.spinner.View()
	}

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

	args := []string{}
	if len(fields) > 1 {
		args = fields[1:]
	}

	switch cmdName {
	case "exit":
		return wrapMsg(ExitCommand{})
	case "save":
		save := SaveCommand{}
		if len(args) > 0 {
			save.Path = args[0]
		}

		return wrapMsg(save)
	default:
		return wrapMsg(UnknownCommand{
			Command: cmdName,
			Args:    args,
		})
	}
}

func wrapMsg(msg tea.Msg) tea.Cmd {
	return func() tea.Msg {
		return msg
	}
}
