package planpicker

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/cneill/smoke/pkg/plan"
)

type Message interface {
	isPlanPickerMessage()
}

type messageType struct{}

func (m messageType) isPlanPickerMessage() {}

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
	styles   Styles
}

func New(plans []plan.Metadata, width, height int) *Model {
	return &Model{
		plans:  plans,
		width:  width,
		height: height,
		styles: InitStyles(),
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
	var sb strings.Builder

	fmt.Fprintln(&sb, m.styles.Title.Render("Select a saved plan"))
	fmt.Fprintln(&sb, m.styles.Help.Render("Use ↑/↓ or j/k to move, Enter to resume, Esc to cancel."))
	fmt.Fprintln(&sb)

	if len(m.plans) == 0 {
		fmt.Fprintln(&sb, m.styles.Empty.Render("No saved plans for this project."))
		return m.styles.SizedContainer(m.width).Render(sb.String())
	}

	start, end := m.visiblePlanRange()
	for index := start; index < end; index++ {
		fmt.Fprintln(&sb, m.renderPlan(index))
	}

	if end-start < len(m.plans) {
		fmt.Fprintln(&sb)
		fmt.Fprintln(&sb, m.styles.Count.Render(fmt.Sprintf("Showing %d-%d of %d saved plans", start+1, end, len(m.plans))))
	}

	return m.styles.SizedContainer(m.width).Render(sb.String())
}

const (
	// chromeLines is the number of lines used by everything except the plan list: container border (2) + container
	// padding (2) + title/help/blank (3) + blank/count footer (2) + trailing newline from the last Fprintln (1).
	chromeLines = 10
	// linesPerPlan is the maximum number of lines a single plan entry takes up (name + log path).
	linesPerPlan = 2
)

func (m *Model) visiblePlanRange() (int, int) {
	limit := min(len(m.plans), max(1, (m.height-chromeLines)/linesPerPlan))
	start := max(0, m.selected-limit+1)

	return start, start + limit
}

func (m *Model) renderPlan(index int) string {
	selected := index == m.selected

	cursor := "  "
	if selected {
		cursor = m.styles.Cursor.Render("➜ ")
	}

	plan := m.plans[index]

	line := cursor + m.styles.PlanNameStyle(selected).Render(plan.DisplayName())
	if plan.LogPath != "" {
		line += "\n  " + m.styles.LogPathStyle(selected).Render("  "+plan.LogPath)
	}

	return m.styles.ItemStyle(selected).Render(line)
}
