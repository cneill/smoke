package history

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
	"github.com/cneill/smoke/pkg/llms"
	"github.com/mattn/go-runewidth"
	"github.com/muesli/reflow/wordwrap"
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
	wasAtBottom := m.viewport.AtBottom()

	switch msg := msg.(type) {
	case ContentUpdate:
		m.log = append(m.log, msg.Message)
		m.viewport.SetContent(m.logContent())
		// only scroll down if we were already at the bottom before updating the history
		if wasAtBottom {
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
				useMD    bool
			)

			switch item.Role {
			case llms.RoleUser:
				roleStr = "👤 User:"
				curStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#0087ff"))
				useMD = true
			case llms.RoleAssistant:
				roleStr = "🤖 Assistant:"
				curStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#00af00"))
				useMD = true
			case llms.RoleTool:
				roleStr = "🔧 Tool:"
				curStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#00afaf"))
				useMD = false
			case llms.RoleSystem:
				roleStr = "🖥️ System"
				curStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#af00af"))
				useMD = false
			case llms.RoleUnknown:
				roleStr = "❓ UNKNOWN ROLE:"
				curStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#af0000"))
				useMD = false
			}

			fmt.Fprint(builder, m.renderBubble(roleStr, curStyle, content, useMD))

		case error:
			header := "⛔ Error:"
			style := lipgloss.NewStyle().Foreground(lipgloss.Color("#af0000"))
			fmt.Fprint(builder, m.renderBubble(header, style, item.Error(), false))
		}

		builder.WriteRune('\n')
	}

	return builder.String()
}

func (m *Model) renderBubble(header string, style lipgloss.Style, content string, useMarkdown bool) string {
	builder := &strings.Builder{}

	bubbleWidth := 40
	line := strings.Repeat("─", bubbleWidth)
	roleWidth := runewidth.StringWidth(header)
	paddingLeft := (bubbleWidth - roleWidth) / 2
	paddingRight := paddingLeft

	if (bubbleWidth-roleWidth)%2 != 0 {
		paddingRight++
	}

	fmt.Fprintln(builder, style.Render("╭"+line+"╮"))
	fmt.Fprintln(builder, style.Render(fmt.Sprintf("│%*s%s%*s│", paddingLeft, "", header, paddingRight, "")))
	fmt.Fprintln(builder, style.Render("╰"+line+"╯"))

	if useMarkdown {
		if mdContent, err := m.mdRenderer.Render(content); err == nil {
			content = mdContent
		}
	}

	fmt.Fprintln(builder, wordwrap.String(content, m.viewport.Width))

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
