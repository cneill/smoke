// Package history contains a Bubbletea model for holding onto LLM chat history and other items like errors, tool calls,
// prompt commands, etc.
package history

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
	"github.com/cneill/smoke/pkg/commands"
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

	initContent string
	log         *Log

	pendingG  bool
	lastGTime time.Time
}

func New(opts *Opts) (*Model, error) {
	if err := opts.OK(); err != nil {
		return nil, fmt.Errorf("options error: %w", err)
	}

	mdRenderer, err := getGlamourRenderer(opts.Width)
	if err != nil {
		return nil, err
	}

	model := &Model{
		viewport:   getViewport(opts),
		mdRenderer: mdRenderer,

		initContent: opts.InitContent,
		log:         NewLog(),
	}

	return model, nil
}

func getViewport(opts *Opts) viewport.Model {
	newViewport := viewport.New(opts.Width, opts.Height)
	newViewport.SetContent(opts.InitContent)

	keyMap := viewport.DefaultKeyMap()
	keyMap.PageUp = key.NewBinding(
		key.WithKeys("pgup", "ctrl+b"),
		key.WithHelp("pgup/ctrl+b", "page up"),
	)
	keyMap.PageDown = key.NewBinding(
		key.WithKeys("pgdown", "ctrl+f"),
		key.WithHelp("pgdown/ctrl+f", "page down"),
	)

	newViewport.KeyMap = keyMap

	return newViewport
}

func getGlamourRenderer(width int) (*glamour.TermRenderer, error) {
	renderer, err := glamour.NewTermRenderer(
		glamour.WithWordWrap(width),
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
		m.log.AddMessage(msg.Message)
		m.viewport.SetContent(m.logContent())
		// only scroll down if we were already at the bottom before updating the history
		if wasAtBottom {
			m.viewport.GotoBottom()
		}
	case ContentRefresh:
		m.log.RefreshLog(msg.Log)

		if logContent := m.logContent(); strings.TrimSpace(logContent) != "" {
			m.viewport.SetContent(logContent)
		} else {
			m.viewport.SetContent(m.initContent)
		}

		if wasAtBottom {
			m.viewport.GotoBottom()
		}
	case tea.KeyMsg:
		return m, m.handleKeyMsg(msg)

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

func (m *Model) handleKeyMsg(msg tea.KeyMsg) tea.Cmd {
	var cmd tea.Cmd

	// Handle VIM-like scrolling commands
	switch msg.String() {
	case "G":
		m.viewport.GotoBottom()
		m.pendingG = false
	case "g":
		if m.pendingG && time.Since(m.lastGTime) <= time.Second {
			m.viewport.GotoTop()
			m.pendingG = false
		} else {
			m.lastGTime = time.Now()
			m.pendingG = true
		}
	default:
		m.pendingG = false
		m.viewport, cmd = m.viewport.Update(msg)
	}

	return cmd
}

func (m *Model) logContent() string {
	builder := &strings.Builder{}

	for _, item := range m.log.Messages() {
		info := bubbleInfo{
			titleStyle: lipgloss.NewStyle().
				Background(lipgloss.Color("#000000")),
			subtitleStyle: lipgloss.NewStyle().
				Background(lipgloss.Color("#000000")).
				Foreground(lipgloss.Color("#444444")).
				Italic(true),
			useMarkdown: false,
		}

		switch item := item.(type) {
		case *llms.Message:
			info = renderLLMMessage(item, info)
		case commands.HistoryUpdateMessage, commands.SessionUpdateMessage, commands.PlanningModeMessage, commands.ReviewModeMessage:
			info = renderCommandMessage(item, info)

		case error:
			info.title = "⛔ Error"
			info.titleStyle = info.titleStyle.
				Foreground(lipgloss.Color("#af0000"))
			info.content = item.Error()

		case string:
			info.title = "Unknown message"
			info.titleStyle = info.titleStyle.Foreground(lipgloss.Color("#999999"))
			info.content = item
		}

		fmt.Fprint(builder, m.renderBubble(info))

		builder.WriteRune('\n')
	}

	return builder.String()
}

func renderLLMMessage(msg *llms.Message, info bubbleInfo) bubbleInfo {
	info.content = msg.Content
	info.subtitle = msg.Added.Format(time.DateTime)

	switch msg.Role {
	case llms.RoleUser:
		info.title = "👤 User"
		info.titleStyle = info.titleStyle.
			Foreground(lipgloss.Color("#0087ff"))
		info.useMarkdown = true
	case llms.RoleAssistant:
		info.title = fmt.Sprintf("🤖 %s (%s)", msg.LLMInfo.Type, msg.LLMInfo.ModelName)
		info.titleStyle = info.titleStyle.
			Foreground(lipgloss.Color("#00af00"))
		info.useMarkdown = true

		if len(msg.ToolsCalled) > 0 {
			info.content += fmt.Sprintf("\n\nTools called: %s\n\n", strings.Join(msg.ToolsCalled, ", "))
		}
	case llms.RoleTool:
		info.title = "🔧 Tool"
		info.titleStyle = info.titleStyle.
			Foreground(lipgloss.Color("#00afaf"))

		if len(msg.ToolsCalled) > 0 {
			info.content = fmt.Sprintf("\nTool call args: %s\n", msg.ToolCallArgs.String()) + info.content
			info.content = fmt.Sprintf("\nTools called: %s\n", strings.Join(msg.ToolsCalled, ", ")) + info.content
		}
	case llms.RoleSystem:
		info.title = "🖥️ System"
		info.titleStyle = info.titleStyle.
			Foreground(lipgloss.Color("#af00af"))
	case llms.RoleUnknown:
		info.title = "❓ UNKNOWN ROLE"
		info.titleStyle = info.titleStyle.
			Foreground(lipgloss.Color("#af0000"))
	}

	return info
}

func renderCommandMessage(msg any, info bubbleInfo) bubbleInfo {
	switch msg := msg.(type) {
	case commands.HistoryUpdateMessage:
		info.title = msg.PromptCommand.Command + " command result"
		info.titleStyle = info.titleStyle.
			Foreground(lipgloss.Color("#dd9911"))
		info.content = msg.Message
		info.useMarkdown = true

	case commands.SessionUpdateMessage:
		switch msg.PromptCommand.Command {
		case commands.CommandSession:
			info.title = "Started new session"
		case commands.CommandLoad:
			sessionFile := "<unknown>"

			if len(msg.PromptCommand.Args) > 0 {
				sessionFile = msg.PromptCommand.Args[0]
			}

			info.title = "Loaded session from file " + sessionFile
		default:
			info.title = "Updated session"
		}

		info.titleStyle = info.titleStyle.
			Foreground(lipgloss.Color("#ffffff"))
		info.content = msg.Message

	case commands.PlanningModeMessage:
		if msg.Enabled {
			info.titleStyle = info.titleStyle.
				Foreground(lipgloss.Color("#550011"))
		} else {
			info.titleStyle = info.titleStyle.
				Foreground(lipgloss.Color("#005511"))
		}

		info.title = "Planning mode"
		info.subtitle = msg.Message

		// TODO: handle mode messages more elegantly
	case commands.ReviewModeMessage:
		if msg.Enabled {
			info.titleStyle = info.titleStyle.
				Foreground(lipgloss.Color("#550011"))
		} else {
			info.titleStyle = info.titleStyle.
				Foreground(lipgloss.Color("#005511"))
		}

		info.title = "Review mode"
		info.subtitle = msg.Message
	}

	return info
}

type bubbleInfo struct {
	title         string
	titleStyle    lipgloss.Style
	subtitle      string
	subtitleStyle lipgloss.Style
	content       string
	useMarkdown   bool
}

// renderBubble displays messages and errors with a nice title/subtitle bubble before the item's content. It word-wraps
// the content of the actual message to ensure it doesn't run off the screen.
func (m *Model) renderBubble(info bubbleInfo) string {
	builder := &strings.Builder{}
	content := info.content
	bubbleWidth := 64
	line := strings.Repeat("─", bubbleWidth)

	titleWidth := runewidth.StringWidth(info.title)
	titlePaddingLeft := (bubbleWidth - titleWidth) / 2
	titlePaddingRight := titlePaddingLeft

	if (bubbleWidth-titleWidth)%2 != 0 {
		titlePaddingRight++
	}

	subtitleWidth := runewidth.StringWidth(info.subtitle)
	subtitlePaddingLeft := (bubbleWidth - subtitleWidth) / 2
	subtitlePaddingRight := subtitlePaddingLeft

	if (bubbleWidth-subtitleWidth)%2 != 0 {
		subtitlePaddingRight++
	}

	fmt.Fprintln(builder, info.titleStyle.Render("╭"+line+"╮"))
	fmt.Fprintln(builder, info.titleStyle.Render(fmt.Sprintf("│%*s%s%*s│", titlePaddingLeft, "", info.title, titlePaddingRight, "")))

	if info.subtitle != "" {
		fmt.Fprint(builder, info.titleStyle.Render(fmt.Sprintf("│%*s", subtitlePaddingLeft, "")))
		fmt.Fprintf(builder, "%s", info.subtitleStyle.Render(info.subtitle))
		fmt.Fprintln(builder, info.titleStyle.Render(fmt.Sprintf("%*s│", subtitlePaddingRight, "")))
	}

	fmt.Fprintln(builder, info.titleStyle.Render("╰"+line+"╯"))

	if info.useMarkdown {
		if mdContent, err := m.mdRenderer.Render(content); err == nil {
			content = mdContent
		}
	} else {
		content = wordwrap.String(content, m.viewport.Width)
	}

	fmt.Fprintln(builder, content)

	return builder.String()
}
