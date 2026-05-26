package history

import (
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/cneill/smoke/internal/uimsg"
	"github.com/cneill/smoke/pkg/commands"
	"github.com/cneill/smoke/pkg/commands/handlers/load"
	"github.com/cneill/smoke/pkg/commands/handlers/mode"
	"github.com/cneill/smoke/pkg/commands/handlers/session"
	"github.com/cneill/smoke/pkg/elicit"
	"github.com/cneill/smoke/pkg/llms"
	"github.com/cneill/smoke/pkg/utils"
)

type bubbleContentKind uint8

const (
	bubbleContentPlainText bubbleContentKind = iota
	bubbleContentMarkdown
	bubbleContentStructured
)

type BubbleContent struct {
	kind       bubbleContentKind
	text       string
	structured *uimsg.HistoryContent
}

func PlainTextContent(text string) BubbleContent {
	return BubbleContent{kind: bubbleContentPlainText, text: text}
}

func MarkdownContent(text string) BubbleContent {
	return BubbleContent{kind: bubbleContentMarkdown, text: text}
}

func StructuredContent(content *uimsg.HistoryContent) BubbleContent {
	return BubbleContent{kind: bubbleContentStructured, structured: content}
}

// Bubble pairs a BubbleStyle with the runtime content to be rendered.
type Bubble struct {
	Style        BubbleStyle
	TitleText    string
	SubtitleText string
	Content      BubbleContent
}

func (b Bubble) ContentText() string {
	return b.Content.text
}

func (b Bubble) ContentIsMarkdown() bool {
	return b.Content.kind == bubbleContentMarkdown
}

func (b Bubble) ContentIsStructured() bool {
	return b.Content.kind == bubbleContentStructured
}

func (b Bubble) StructuredContent() *uimsg.HistoryContent {
	return b.Content.structured
}

// BubbleForHistoryItem takes any item in the history log and translates it into a Bubble appropriate for rendering.
func BubbleForHistoryItem(item any, styles Styles) Bubble {
	switch item := item.(type) {
	case *llms.Message:
		return bubbleForLLMMessage(item, styles)
	case commands.Message:
		return bubbleForCommandMessage(item, styles)
	case elicit.Message:
		return bubbleForElicitMessage(item, styles)
	case *uimsg.Error:
		return Bubble{
			Style:     styles.ErrorBubble,
			TitleText: "⛔ Error",
			Content:   PlainTextContent(item.Error()),
		}
	case string:
		return Bubble{
			Style:     styles.UnknownBubble,
			TitleText: "❓ Unknown message",
			Content:   PlainTextContent(item),
		}
	default:
		return unsupportedHistoryItemBubble(styles, item)
	}
}

func assistantRoleContentText(msg *llms.Message, contentText string) string {
	if !msg.HasToolCalls() {
		return contentText
	}

	var sb strings.Builder
	sb.WriteString(contentText)

	if strings.TrimSpace(contentText) != "" {
		sb.WriteString("\n\n")
	}

	sb.WriteString("Tool calls:\n")

	for _, toolCall := range msg.ToolCalls {
		fmt.Fprintf(&sb, "- `%s`: `%s`\n", toolCall.Name, toolCall.Args.String())
	}

	return sb.String()
}

func toolRoleContentText(msg *llms.Message, contentText string) (string, error) {
	if n := len(msg.ToolCalls); n != 1 {
		return "", fmt.Errorf("invalid number of calls (%d) in tool role message; expecting 1", n)
	}

	var sb strings.Builder
	sb.WriteString(contentText)

	if strings.TrimSpace(contentText) != "" {
		sb.WriteString("\n\n")
	}

	fmt.Fprintf(&sb, "Tool call to %q with args: %s", msg.ToolCalls[0].Name, msg.ToolCalls[0].Args.String())

	return sb.String(), nil
}

func bubbleForLLMMessage(msg *llms.Message, styles Styles) Bubble {
	var (
		style       BubbleStyle
		titleText   string
		contentText = msg.TextContent
	)

	if err := msg.OK(); err != nil {
		return invalidLLMMessageBubble(msg, styles, err)
	}

	switch msg.Role {
	case llms.RoleUser:
		style = styles.UserBubble
		titleText = "👤 User"
	case llms.RoleAssistant:
		style = styles.AssistantBubble
		titleText = fmt.Sprintf("🤖 %s (%s)", msg.LLMInfo.Type, msg.LLMInfo.ModelName)
		contentText = assistantRoleContentText(msg, contentText)
	case llms.RoleTool:
		style = styles.ToolBubble
		titleText = "🔧 Tool"

		text, err := toolRoleContentText(msg, contentText)
		if err != nil {
			return invalidLLMMessageBubble(msg, styles, err)
		}

		contentText = text
	case llms.RoleSystem:
		style = styles.SystemBubble
		titleText = "🖥️ System"
	case llms.RoleUnknown:
		style = styles.UnknownBubble
		titleText = "❓ Unknown Role"
	}

	return Bubble{
		Style:        style,
		TitleText:    titleText,
		SubtitleText: msg.Added.Format(time.DateTime),
		Content:      llmBubbleContent(msg, contentText),
	}
}

func invalidLLMMessageBubble(msg *llms.Message, styles Styles, err error) Bubble {
	slog.Error("got invalid LLM message in history", "message", msg, "error", err)

	return internalErrorBubble(styles, fmt.Sprintf("invalid LLM message in history: %v", err))
}

func bubbleForCommandMessage(msg commands.Message, styles Styles) Bubble {
	switch msg := msg.(type) {
	case commands.HistoryUpdateMessage:
		return Bubble{
			Style:     styles.CommandBubble,
			TitleText: fmt.Sprintf("⚙️ %s command result", utils.TitleCase(msg.PromptMessage.Command)),
			Content:   commandHistoryContent(msg),
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
			Style:     styles.SessionBubble,
			TitleText: titleText,
			Content:   PlainTextContent(msg.Message),
		}

	case mode.Message:
		modeTitle := utils.TitleCase(msg.Mode)

		return Bubble{
			Style:        styles.SessionBubble,
			TitleText:    modeTitle + " mode",
			SubtitleText: msg.Message,
		}
	default:
		return unsupportedMessageBubble(styles, "commands.Message", msg)
	}
}

func bubbleForElicitMessage(msg elicit.Message, styles Styles) Bubble {
	switch msg := msg.(type) {
	case elicit.RequestMessage:
		return Bubble{
			Style:     styles.ElicitBubble,
			TitleText: "Question",
			Content:   PlainTextContent(msg.String()),
		}
	case elicit.UserCanceledMessage:
		return Bubble{
			Style:     styles.ElicitCanceledBubble,
			TitleText: "Canceled",
			Content:   PlainTextContent(msg.String()),
		}
	case elicit.UserResponseMessage:
		return Bubble{
			Style:     styles.ElicitBubble,
			TitleText: "Response",
			Content:   PlainTextContent(msg.String()),
		}
	default:
		return unsupportedMessageBubble(styles, "elicit.Message", msg)
	}
}

func llmBubbleContent(msg *llms.Message, contentText string) BubbleContent {
	switch msg.Role {
	case llms.RoleUser, llms.RoleAssistant, llms.RoleSystem:
		return MarkdownContent(contentText)
	case llms.RoleTool, llms.RoleUnknown:
		return PlainTextContent(contentText)
	default:
		return PlainTextContent(contentText)
	}
}

func commandHistoryContent(msg commands.HistoryUpdateMessage) BubbleContent {
	if msg.Content != nil {
		return StructuredContent(msg.Content)
	}

	return MarkdownContent(msg.Message)
}

func unsupportedHistoryItemBubble(styles Styles, item any) Bubble {
	slog.Error("unsupported history item type", "item", item, "type", fmt.Sprintf("%T", item))

	return internalErrorBubble(styles, fmt.Sprintf("history received unsupported message type %T", item))
}

func unsupportedMessageBubble(styles Styles, interfaceName string, msg any) Bubble {
	slog.Error("unsupported interface implementation", "interface", interfaceName, "value", msg, "type", fmt.Sprintf("%T", msg))

	return internalErrorBubble(styles, fmt.Sprintf("unsupported %s type %T", interfaceName, msg))
}

func internalErrorBubble(styles Styles, text string) Bubble {
	return Bubble{
		Style:     styles.ErrorBubble,
		TitleText: "⛔ Internal error",
		Content:   PlainTextContent(text),
	}
}
