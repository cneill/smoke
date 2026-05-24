// Package history contains a Bubbletea model for holding onto LLM chat history and other items like errors, tool calls,
// prompt commands, etc.
package history

import (
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/cneill/smoke/internal/uimsg"
	"github.com/cneill/smoke/pkg/commands"
	"github.com/cneill/smoke/pkg/commands/handlers/load"
	"github.com/cneill/smoke/pkg/commands/handlers/mode"
	"github.com/cneill/smoke/pkg/commands/handlers/session"
	"github.com/cneill/smoke/pkg/elicit"
	"github.com/cneill/smoke/pkg/llms"
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
	styles     Styles

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
		styles:     InitStyles(),

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
		glamour.WithPreservedNewLines(),
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
		var bubble Bubble

		switch item := item.(type) {
		case *llms.Message:
			bubble = renderLLMMessage(item, m.styles)
		case commands.Message:
			bubble = renderCommandMessage(item, m.styles)
		case elicit.Message:
			bubble = renderElicitMessage(item, m.styles)
		case *uimsg.Error:
			bubble = Bubble{
				Style:       m.styles.ErrorBubble,
				TitleText:   "⛔ Error",
				ContentText: item.Error(),
			}

		case string:
			bubble = Bubble{
				Style:       m.styles.UnknownBubble,
				TitleText:   "Unknown message",
				ContentText: item,
			}

		default:
			slog.Error("UNKNOWN MESSAGE TYPE", "item", item, "type", fmt.Sprintf("%T", item))
			continue
		}

		fmt.Fprint(builder, m.renderBubble(bubble))

		builder.WriteRune('\n')
	}

	return builder.String()
}

func renderAssistantToolCallSummary(msg *llms.Message, contentText string) string {
	if !msg.HasToolCalls() {
		return contentText
	}

	return contentText + "\n\nTools called: " + strings.Join(msg.ToolCalls.Names(), ", ") + "\n"
}

func renderToolMessageDetails(msg *llms.Message, contentText string) string {
	if !msg.HasToolCalls() {
		return contentText
	}

	// This message should only have 1 tool call, but we check just in case
	toolCallDetails := make([]string, 0, len(msg.ToolCalls))
	for _, toolCall := range msg.ToolCalls {
		toolCallDetails = append(toolCallDetails, fmt.Sprintf("Tool call to %q with args: %s", toolCall.Name, toolCall.Args.String()))
	}

	if len(toolCallDetails) == 0 {
		return contentText
	}

	contentText = strings.TrimSpace(contentText)
	if contentText != "" {
		contentText += "\n\n"
	}

	contentText += strings.Join(toolCallDetails, "\n")

	return contentText
}

func renderLLMMessage(msg *llms.Message, styles Styles) Bubble {
	var (
		style       BubbleStyle
		titleText   string
		contentText = msg.TextContent
	)

	switch msg.Role {
	case llms.RoleUser:
		style = styles.UserBubble
		titleText = "👤 User"
	case llms.RoleAssistant:
		style = styles.AssistantBubble
		titleText = fmt.Sprintf("🤖 %s (%s)", msg.LLMInfo.Type, msg.LLMInfo.ModelName)
		contentText = renderAssistantToolCallSummary(msg, contentText)
	case llms.RoleTool:
		style = styles.ToolBubble
		titleText = "🔧 Tool"
		contentText = renderToolMessageDetails(msg, contentText)
	case llms.RoleSystem:
		style = styles.SystemBubble
		titleText = "🖥️ System"
	case llms.RoleUnknown:
		style = styles.ErrorBubble
		titleText = "❓ UNKNOWN ROLE"
	}

	return Bubble{
		Style:        style,
		TitleText:    titleText,
		SubtitleText: msg.Added.Format(time.DateTime),
		ContentText:  contentText,
	}
}

func renderCommandMessage(msg commands.Message, styles Styles) Bubble {
	switch msg := msg.(type) {
	case commands.HistoryUpdateMessage:
		style := styles.CommandBubble
		// TODO: make sure this makes sense?
		style.UseMarkdown = true

		return Bubble{
			Style:       style,
			TitleText:   msg.PromptMessage.Command + " command result",
			ContentText: msg.Message,
		}

	case commands.SessionUpdateMessage:
		var titleText string

		switch msg.PromptMessage.Command {
		case session.Name:
			titleText = "Started new session"
		case load.Name:
			sessionFile := "<unknown>"

			if len(msg.PromptMessage.Args) > 0 {
				sessionFile = msg.PromptMessage.Args[0]
			}

			titleText = "Loaded session from file " + sessionFile
		default:
			titleText = "Updated session"
		}

		return Bubble{
			Style:       styles.SessionBubble,
			TitleText:   titleText,
			ContentText: msg.Message,
		}

	case mode.Message:
		modeTitle := strings.Title(string(msg.Mode)) //nolint:staticcheck

		return Bubble{
			Style:        styles.SessionBubble,
			TitleText:    modeTitle + " mode",
			SubtitleText: msg.Message,
		}
	}

	return Bubble{}
}

func renderElicitMessage(msg elicit.Message, styles Styles) Bubble {
	switch msg := msg.(type) {
	case elicit.RequestMessage:
		return Bubble{
			Style:       styles.ElicitBubble,
			TitleText:   "Question",
			ContentText: msg.String(),
		}

	case elicit.UserCanceledMessage:
		return Bubble{
			Style:       styles.ElicitCanceledBubble,
			TitleText:   "Canceled",
			ContentText: msg.String(),
		}

	case elicit.UserResponseMessage:
		return Bubble{
			Style:       styles.ElicitBubble,
			TitleText:   "Response",
			ContentText: msg.String(),
		}
	}

	return Bubble{}
}

// Bubble pairs a BubbleStyle with the runtime content to be rendered.
type Bubble struct {
	Style        BubbleStyle
	TitleText    string
	SubtitleText string
	ContentText  string
}

// renderBubble displays messages and errors with a nice title/subtitle bubble before the item's content. It word-wraps
// the content of the actual message to ensure it doesn't run off the screen.
func (m *Model) renderBubble(bubble Bubble) string {
	builder := &strings.Builder{}
	content := bubble.ContentText
	headerParts := []string{bubble.Style.Title.Render(bubble.TitleText)}

	if bubble.SubtitleText != "" {
		headerParts = append(headerParts, bubble.Style.Subtitle.Render(bubble.SubtitleText))
	}

	header := bubble.Style.Container.Render(strings.Join(headerParts, "\n"))

	fmt.Fprintln(builder, header)

	if bubble.Style.UseMarkdown {
		if mdContent, err := m.mdRenderer.Render(content); err == nil {
			content = mdContent
		}
	} else {
		content = wordwrap.String(content, m.viewport.Width)
	}

	fmt.Fprintln(builder, bubble.Style.Content.Render(content))

	return builder.String()
}
