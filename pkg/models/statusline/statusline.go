package statusline

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/cneill/smoke/pkg/llmctx/modes"
	"github.com/cneill/smoke/pkg/llms"
	"github.com/cneill/smoke/pkg/smoke"
	"github.com/cneill/smoke/pkg/utils"
)

type Model struct {
	focused             bool
	modelMode           modes.Mode
	width               int
	contextWindowTokens int64
	modelInfo           llms.ModelInfo
	styles              Styles
}

func New(width int, modelInfo llms.ModelInfo) *Model {
	model := &Model{
		focused:   true,
		modelMode: modes.ModeWork,
		width:     width,
		modelInfo: modelInfo,
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
	case smoke.UsageUpdateMessage:
		m.contextWindowTokens = msg.ContextWindowTokens
	case smoke.ModeMessage:
		m.modelMode = msg.Mode
	}

	return m, tea.Batch(commands...)
}

func (m *Model) View() string {
	style := m.styleVariant()

	separator := style.Border.Render(" ✱ ")

	modelStyled := style.Usage.Render(fmt.Sprintf("%s/%s", m.modelInfo.Provider, m.modelInfo.Model))
	modeStyled := style.Usage.Render(fmt.Sprintf("mode: %s", m.modelMode))
	left := modelStyled + separator + modeStyled
	leftWidth := lipgloss.Width(left)

	usage := separator + style.Usage.Render("ctx: "+utils.CommaFormatInt(m.contextWindowTokens))

	// Models without a known context window size (e.g. Ollama) would otherwise render a 0 max and a NaN/Inf percentage.
	if maxContextWindow := m.modelInfo.ContextWindowTokens; maxContextWindow > 0 {
		percentage := float64(m.contextWindowTokens) / float64(maxContextWindow) * 100
		usage += style.Border.Render(" / ") + style.Usage.Render(utils.CommaFormatInt(maxContextWindow))
		usage += style.Usage.Render(fmt.Sprintf(" (%.2f%%)", percentage))
	}

	usage += style.Usage.Render(" ") + " "
	usageWidth := lipgloss.Width(usage)

	borderWidth := max(0, m.width-leftWidth-usageWidth)
	border := style.Border.Render(strings.Repeat(" ", borderWidth))

	return border + left + usage
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
