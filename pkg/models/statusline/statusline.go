package statusline

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/cneill/smoke/pkg/llmctx/modes"
	"github.com/cneill/smoke/pkg/smoke"
	"github.com/cneill/smoke/pkg/utils"
)

type Model struct {
	focused             bool
	modelMode           modes.Mode
	width               int
	contextWindowTokens int64
	maxContextWindow    int64
	styles              Styles
}

func New(width int, maxContextWindow int64) *Model {
	model := &Model{
		focused:          true,
		modelMode:        modes.ModeWork,
		width:            width,
		maxContextWindow: maxContextWindow,
		styles:           InitStyles(),
	}

	return model
}

func (m *Model) Init() tea.Cmd {
	return nil
}

func (m *Model) Update(msg tea.Msg) (*Model, tea.Cmd) {
	commands := []tea.Cmd{}

	switch msg := msg.(type) {
	case smoke.UsageUpdateMessage:
		m.contextWindowTokens = msg.ContextWindowTokens
	case smoke.ModeMessage:
		m.modelMode = msg.Mode
	}

	return m, tea.Batch(commands...)
}

func (m *Model) View() string {
	style := m.styleVariant()

	modeStyled := style.Usage.Render(fmt.Sprintf("mode: %s", m.modelMode))
	modeWidth := lipgloss.Width(modeStyled)

	usagePadding := style.Border.Render(" ✱ ")

	usageStyled := style.Usage.Render("ctx: " + utils.CommaFormatInt(m.contextWindowTokens))
	maxStyled := style.Border.Render(" / ") + style.Usage.Render(utils.CommaFormatInt(m.maxContextWindow))
	percentage := float64(m.contextWindowTokens) / float64(m.maxContextWindow) * 100
	percentageStyled := style.Usage.Render(fmt.Sprintf(" (%.2f%%) ", percentage))
	usage := usagePadding + usageStyled + maxStyled + percentageStyled + " "
	usageWidth := lipgloss.Width(usage)

	borderWidth := max(0, m.width-modeWidth-usageWidth)
	border := style.Border.Render(strings.Repeat(" ", borderWidth))

	return border + modeStyled + usage
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
