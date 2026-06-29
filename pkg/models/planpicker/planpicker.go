package planpicker

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/cneill/smoke/pkg/plan"
)

type Message interface {
	isPlanPickerMessage()
}

type messageType struct{}

func (messageType) isPlanPickerMessage() {}

type SelectedMessage struct {
	messageType

	Plan plan.Metadata
}

type CanceledMessage struct {
	messageType
}

type Model struct {
	plans    []plan.Metadata
	selected int
	width    int
	height   int
}

func New(plans []plan.Metadata, width, height int) *Model {
	return &Model{
		plans:  plans,
		width:  width,
		height: height,
	}
}

func (m *Model) Init() tea.Cmd { return nil }

func (m *Model) Update(msg tea.Msg) (*Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	case tea.KeyMsg:
		switch msg.String() {
		case "esc", "ctrl+c", "q":
			return m, func() tea.Msg { return CanceledMessage{} }
		case "enter":
			if len(m.plans) == 0 {
				return m, func() tea.Msg { return CanceledMessage{} }
			}

			selected := m.plans[m.selected]

			return m, func() tea.Msg { return SelectedMessage{Plan: selected} }
		case "up", "k":
			if m.selected > 0 {
				m.selected--
			}
		case "down", "j":
			if m.selected < len(m.plans)-1 {
				m.selected++
			}
		}
	}

	return m, nil
}

func (m *Model) View() string {
	var builder strings.Builder

	title := lipgloss.NewStyle().Bold(true).Render("Select a saved plan")
	fmt.Fprintln(&builder, title)
	fmt.Fprintln(&builder, "Use ↑/↓ or j/k to move, Enter to resume, Esc to cancel.")
	fmt.Fprintln(&builder)

	if len(m.plans) == 0 {
		fmt.Fprintln(&builder, "No saved plans for this project.")
		return builder.String()
	}

	limit := len(m.plans)
	if m.height > 4 && limit > m.height-4 {
		limit = m.height - 4
	}

	for index := range limit {
		prefix := "  "
		lineStyle := lipgloss.NewStyle()

		if index == m.selected {
			prefix = "> "
			lineStyle = lineStyle.Bold(true)
		}

		plan := m.plans[index]

		line := fmt.Sprintf("%s%s", prefix, plan.DisplayName())
		if plan.LogPath != "" {
			line += "\n    " + plan.LogPath
		}

		fmt.Fprintln(&builder, lineStyle.Render(line))
	}

	return builder.String()
}
