package statusline

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/cneill/smoke/pkg/llmctx/modes"
	"github.com/cneill/smoke/pkg/smoke"
)

type Model struct {
	focused        bool
	modelMode      modes.Mode
	width          int
	completionText string
	inputTokens    int64
	outputTokens   int64
	styles         Styles
}

func New(width int) *Model {
	model := &Model{
		focused:   true,
		modelMode: modes.ModeWork,
		width:     width,
		styles:    InitStyles(),
	}

	return model
}

func (m *Model) Init() tea.Cmd {
	return nil
}

func (m *Model) Update(msg tea.Msg) (*Model, tea.Cmd) {
	commands := []tea.Cmd{}

	switch msg := msg.(type) {
	case CompletionMessage:
		m.completionText = msg.Text
	case smoke.UsageUpdateMessage:
		m.inputTokens = msg.InputTokens
		m.outputTokens = msg.OutputTokens
	case smoke.ModeMessage:
		m.modelMode = msg.Mode
	}

	return m, tea.Batch(commands...)
}

func (m *Model) View() string {
	style := m.styleVariant()

	var (
		suggestion      string
		suggestionWidth int
	)

	if m.completionText != "" {
		suggestionStyled := style.Completion.Render(m.completionText)
		suggestionPadding := style.Border.Render(strings.Repeat(" ", 2))
		suggestion = suggestionPadding + suggestionStyled
		suggestionWidth = lipgloss.Width(suggestion)
	}

	modeStyled := style.Usage.Render(fmt.Sprintf("mode: %s", m.modelMode))
	modeWidth := lipgloss.Width(modeStyled)

	usagePadding := style.Border.Render(" ✱ ")

	usageStyled := style.Usage.Render(fmt.Sprintf("in: %d, out: %d", m.inputTokens, m.outputTokens))
	usage := usagePadding + usageStyled + " "
	usageWidth := lipgloss.Width(usage)

	borderWidth := max(0, m.width-modeWidth-usageWidth-suggestionWidth)
	border := style.Border.Render(strings.Repeat(" ", borderWidth))

	return suggestion + border + modeStyled + usage
}

func (m *Model) SetFocus(focused bool) {
	m.focused = focused
}

func (m *Model) SetWidth(width int) {
	m.width = width
}

func (m *Model) styleVariant() Style {
	variant := Focused
	if !m.focused {
		variant = Blurred
	}

	return m.styles.GetVariant(variant)
}
