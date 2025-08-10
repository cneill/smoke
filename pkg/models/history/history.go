package history

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
	"github.com/cneill/smoke/pkg/llms"
)

type Opts struct {
	Width       int
	Height      int
	InitContent string
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

type Model struct {
	viewport   viewport.Model
	mdRenderer *glamour.TermRenderer

	log []any
}

func New(opts *Opts) (*Model, error) {
	if err := opts.OK(); err != nil {
		return nil, fmt.Errorf("options error: %w", err)
	}

	viewport := viewport.New(opts.Width, opts.Height)
	viewport.SetContent(opts.InitContent)

	mdRenderer, err := getGlamourRenderer(opts.Width)
	if err != nil {
		return nil, err
	}

	model := &Model{
		viewport:   viewport,
		mdRenderer: mdRenderer,

		log: []any{},
	}

	return model, nil
}

func getGlamourRenderer(width int) (*glamour.TermRenderer, error) {
	renderer, err := glamour.NewTermRenderer(
		glamour.WithWordWrap(width-4),
		glamour.WithEmoji(),
		glamour.WithAutoStyle(),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to set up markdown renderer: %w", err)
	}

	return renderer, nil
}

func (m *Model) Init() tea.Cmd {
	return nil
}

func (m *Model) Update(msg tea.Msg) (*Model, tea.Cmd) {
	switch msg := msg.(type) {
	case ContentUpdate:
		m.log = append(m.log, msg.Message)
		m.viewport.SetContent(m.logContent())
		// only scroll down if we're already at the bottom
		if m.viewport.AtBottom() {
			m.viewport.GotoBottom()
		}
	default:
		var cmd tea.Cmd
		m.viewport, cmd = m.viewport.Update(msg)

		return m, cmd
	}

	return m, nil
}

func (m *Model) View() string {
	return m.viewport.View()
}

func (m *Model) logContent() string {
	builder := &strings.Builder{}

	for _, item := range m.log {
		switch item := item.(type) {
		case *llms.Message:
			var (
				roleStr  string
				curStyle lipgloss.Style
				content  = item.Content
			)

			switch item.Role {
			case llms.RoleUser:
				roleStr = "👤 User:"
				curStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#0087ff"))

				if mdContent, err := m.mdRenderer.Render(content); err == nil {
					content = mdContent
				}
			case llms.RoleAssistant:
				roleStr = "🤖 Assistant:"
				curStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#00af00"))

				if mdContent, err := m.mdRenderer.Render(content); err == nil {
					content = mdContent
				}
			case llms.RoleTool:
				roleStr = "🔧 Tool:"
				curStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#00afaf"))
			case llms.RoleSystem:
				roleStr = "🛠 System:"
				curStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#af00af"))
			case llms.RoleUnknown:
				roleStr = "❓ UNKNOWN ROLE:"
				curStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#af0000"))
			}

			lines := strings.Split(content, "\n")
			maxWidth := len(roleStr)

			for _, line := range lines {
				if l := len(line); l > maxWidth {
					maxWidth = l
				}
			}

			topBorder := curStyle.Render(fmt.Sprintf("╭%s╮", strings.Repeat("─", maxWidth+2)))
			roleLine := curStyle.Render(fmt.Sprintf("│ %-*s │", maxWidth, roleStr))

			var contentLines strings.Builder
			for _, line := range lines {
				contentLines.WriteString(curStyle.Render(fmt.Sprintf("│ %-*s │", maxWidth, line)))
				contentLines.WriteString("\n")
			}

			bottomBorder := curStyle.Render(fmt.Sprintf("╰%s╯", strings.Repeat("─", maxWidth+2)))

			fmt.Fprintf(builder, "%s\n%s\n%s%s\n\n", topBorder, roleLine, contentLines.String(), bottomBorder)

		case error:
			fmt.Fprintf(builder, "ERROR: %v\n", item)
		}

		builder.WriteRune('\n')
	}

	return builder.String()
}

func (m *Model) Resize(width, height int) {
	m.viewport.Width = width
	m.viewport.Height = height

	// TODO: figure out how to make this reasonably performant....
	// newRenderer, err := getGlamourRenderer(width)
	// if err == nil {
	// 	m.mdRenderer.Close()
	// 	m.mdRenderer = newRenderer
	// }
}

func (m *Model) GetWidth() int {
	return m.viewport.Width
}

func (m *Model) GetHeight() int {
	return m.viewport.Height
}

type ContentUpdate struct {
	Message any
}
