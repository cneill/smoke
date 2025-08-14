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
	log         []any

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
		log:         []any{},
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
		m.log = append(m.log, msg.Message)
		m.viewport.SetContent(m.logContent())
		// only scroll down if we were already at the bottom before updating the history
		if wasAtBottom {
			m.viewport.GotoBottom()
		}
	case ContentRefresh:
		m.log = msg.Log
		if logContent := m.logContent(); strings.TrimSpace(logContent) != "" {
			m.viewport.SetContent(logContent)
		} else {
			m.viewport.SetContent(m.initContent)
		}

		if wasAtBottom {
			m.viewport.GotoBottom()
		}
	case tea.KeyMsg:
		// Handle VIM-like scrolling commands
		switch msg.String() {
		case "G":
			m.viewport.GotoBottom()
			m.pendingG = false

			return m, nil
		case "g":
			if m.pendingG && time.Since(m.lastGTime) <= time.Second {
				m.viewport.GotoTop()
				m.pendingG = false

				return m, nil
			}

			m.pendingG = true
			m.lastGTime = time.Now()

			return m, nil
		default:
			m.pendingG = false

			var cmd tea.Cmd
			m.viewport, cmd = m.viewport.Update(msg)

			return m, cmd
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
			info.content = item.Content
			info.subtitle = item.Added.Format(time.DateTime)

			switch item.Role {
			case llms.RoleUser:
				info.title = "👤 User"
				info.titleStyle = info.titleStyle.
					Foreground(lipgloss.Color("#0087ff"))
				info.useMarkdown = true
			case llms.RoleAssistant:
				info.title = fmt.Sprintf("🤖 %s (%s)", item.LLMInfo.Type, item.LLMInfo.ModelName)
				info.titleStyle = info.titleStyle.
					Foreground(lipgloss.Color("#00af00"))
				info.useMarkdown = true

				if len(item.ToolsCalled) > 0 {
					info.content += fmt.Sprintf("\n\nTools called: %s\n\n", strings.Join(item.ToolsCalled, ", "))
				}
			case llms.RoleTool:
				info.title = "🔧 Tool"
				info.titleStyle = info.titleStyle.
					Foreground(lipgloss.Color("#00afaf"))

				if len(item.ToolsCalled) > 0 {
					info.content = fmt.Sprintf("\nTool call args: %s\n", item.ToolCallArgs.String()) + info.content
					info.content = fmt.Sprintf("\nTools called: %s\n", strings.Join(item.ToolsCalled, ", ")) + info.content
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

		case commands.HistoryUpdateMessage:
			info.title = item.PromptCommand.Command + " command result"
			info.titleStyle = info.titleStyle.
				Foreground(lipgloss.Color("#dd9911"))
			info.content = item.Message

		case commands.SessionUpdateMessage:
			// TODO: bounds-check?
			switch item.PromptCommand.Command {
			case commands.CommandClear:
				info.title = "Cleared session"
			case commands.CommandLoad:
				info.title = "Loaded session from file " + item.PromptCommand.Args[0]
			default:
				info.title = "Updated session"
			}

			info.titleStyle = info.titleStyle.
				Foreground(lipgloss.Color("#ffffff"))
			info.content = item.Message

		case error:
			info.title = "⛔ Error"
			info.titleStyle = info.titleStyle.
				Foreground(lipgloss.Color("#af0000"))
			info.content = item.Error()
		}

		fmt.Fprint(builder, m.renderBubble(info))

		builder.WriteRune('\n')
	}

	return builder.String()
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

// ContentUpdate adds a new message to the log.
type ContentUpdate struct {
	Message any
}

// ContentRefresh replaces the current log with its own log.
type ContentRefresh struct {
	Log []any
}
