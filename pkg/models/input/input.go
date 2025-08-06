package input

import (
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-runewidth"
)

const prompt = "│"

type Model struct {
	textarea textarea.Model
	spinner  spinner.Model

	waiting bool
}

func New(width, height int) (*Model, error) {
	model := &Model{
		textarea: getTextArea(width, height),
		spinner:  getSpinner(height),
	}

	return model, nil
}

func getTextArea(width, height int) textarea.Model {
	model := textarea.New()
	model.Placeholder = "Enter your message."
	model.Focus()

	model.Prompt = prompt
	model.CharLimit = 0

	model.SetWidth(width)
	model.SetHeight(height)
	model.MaxHeight = 5 // TODO: make customizable

	model.FocusedStyle.CursorLine = lipgloss.NewStyle().
		Background(lipgloss.Color("#000000")).
		Foreground(lipgloss.Color("#eeeeee"))
	model.FocusedStyle.Text = lipgloss.NewStyle().
		Background(lipgloss.Color("#000000")).
		Foreground(lipgloss.Color("#eeeeee"))
	model.FocusedStyle.Prompt = lipgloss.NewStyle().
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

	if m.waiting {
		newSpinner, cmd := m.spinner.Update(msg)
		m.spinner = newSpinner

		commands = append(commands, cmd)
	}

	newTextarea, cmd := m.textarea.Update(msg)
	m.textarea = newTextarea

	commands = append(commands, cmd)

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
			extraLines := (lineWidth / inputWidth)
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

type Message struct {
	Underlying tea.Msg
}

type ResizeMessage struct{}
