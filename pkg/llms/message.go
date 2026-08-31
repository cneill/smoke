package llms

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"time"

	"github.com/cneill/smoke/pkg/utils"
)

// Message holds a provider-agnostic representation of a single message from the user, assistant, etc.
//
// When Message's Role is [RoleAssistant], tool call blocks will hold details of _all_ the tool calls made by the
// provider. When its role is [RoleTool], there will be exactly _one_ tool call result matching the original call.
type Message struct {
	ID      string    `json:"id"`
	Added   time.Time `json:"added"`
	Updated time.Time `json:"updated"`
	Role    Role      `json:"role"`
	Blocks  Blocks    `json:"blocks,omitempty"`
	Error   string    `json:"error,omitempty"`
	LLMInfo *LLMInfo  `json:"llm_info,omitempty"`
}

func NewMessage(opts ...MessageOpt) *Message {
	now := time.Now()

	msg := &Message{
		ID:      utils.RandID(16),
		Added:   now,
		Updated: now,
	}

	for _, opt := range opts {
		msg = opt(msg)
	}

	return msg
}

func SimpleMessage(role Role, content string) *Message {
	return NewMessage(
		WithRole(role),
		WithTextContent(content),
	)
}

func (m *Message) OK() error {
	switch {
	case m.ID == "":
		return fmt.Errorf("message is missing ID")
	case m.Role == "":
		return fmt.Errorf("message is missing role")
	case m.Blocks.OK() != nil:
		return fmt.Errorf("message block error: %w", m.Blocks.OK())
	case m.Role == RoleTool:
		if n := len(m.ToolCalls()); n == 0 {
			return fmt.Errorf("message with %q role is missing tool call information", RoleTool)
		} else if n > 1 {
			return fmt.Errorf("message with %q role has too many tool call references (%d), expecting 1", RoleTool, n)
		}
	}

	return nil
}

func (m *Message) Clone() *Message {
	clone := *m

	clone.Blocks = make(Blocks, len(m.Blocks))
	for i := range m.Blocks {
		clone.Blocks[i] = m.Blocks[i].Clone()
	}

	if m.LLMInfo != nil {
		info := *m.LLMInfo
		clone.LLMInfo = &info
	}

	return &clone
}

func (m *Message) Update(opts ...MessageOpt) *Message {
	clone := m.Clone()
	for _, opt := range opts {
		clone = opt(clone)
	}

	clone.Updated = time.Now()

	return clone
}

func (m *Message) TextContent() string {
	var builder strings.Builder

	for _, block := range m.Blocks {
		if block.Type() == BlockTypeText {
			builder.WriteString(block.Text.Text)
		}
	}

	return builder.String()
}

func (m *Message) Images() []ImageBlock {
	var result []ImageBlock

	for _, block := range m.Blocks {
		if block.Type() == BlockTypeImage {
			clone := block.Clone()
			result = append(result, *clone.Image)
		}
	}

	return result
}

func (m *Message) ToolCalls() ToolCalls {
	var result ToolCalls

	for _, block := range m.Blocks {
		if block.Type() == BlockTypeToolCall {
			result = append(result, block.ToolCall.ToolCall.Clone())
		}
	}

	return result
}

func (m *Message) ReasoningSummaries() []string {
	var result []string

	for _, block := range m.Blocks {
		if block.Type() == BlockTypeReasoning {
			result = append(result, block.Reasoning.Summary...)
		}
	}

	return result
}

func (m *Message) HasToolCalls() bool { return len(m.ToolCalls()) > 0 }

func (m *Message) LogValue() slog.Value {
	attrs := []slog.Attr{
		slog.String("id", m.ID),
		slog.String("role", string(m.Role)),
		slog.Time("added", m.Added),
	}
	if m.HasToolCalls() {
		attrs = append(attrs, slog.Any("tool_calls", m.ToolCalls()))
	}

	if m.Error != "" {
		attrs = append(attrs, slog.String("error", m.Error))
	}

	if m.LLMInfo != nil {
		attrs = append(attrs, slog.Any("llm_info", m.LLMInfo))
	}

	attrs = append(attrs, slog.String("content", m.TextContent()))

	return slog.GroupValue(attrs...)
}

func (m *Message) ToMarkdown() string {
	builder := &strings.Builder{}
	fmt.Fprintf(builder, "# %s\n*(%s)*\n\n", m.Role, m.Added.Format(time.RFC1123))

	calls, text := m.ToolCalls(), m.TextContent()
	if len(calls) > 0 {
		for _, call := range calls {
			fmt.Fprintf(builder, "**Tool called:** `%s`\n**Args:** `%s`\n", call.Name, call.ArgsString())
		}

		if text != "" {
			fmt.Fprintf(builder, "**Content:**\n\n%s\n", text)
		}
	} else {
		builder.WriteString(text)
	}

	builder.WriteString("\n\n----\n")

	return builder.String()
}

func (m *Message) ImageB64URL() string {
	images := m.Images()
	if len(images) == 0 {
		return ""
	}

	return "data:" + images[0].MediaType + ";base64," + base64.StdEncoding.EncodeToString(images[0].Data)
}

func (m *Message) UnmarshalJSON(data []byte) error {
	type canonical Message

	// This is for handling backwards compatibility with saved sessions from before we used provider block mechanics.
	// TODO: remove this soon
	var raw struct {
		canonical

		Content *string   `json:"content"`
		Calls   ToolCalls `json:"tool_calls"`
	}

	if err := json.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("unmarshal message: %w", err)
	}

	*m = Message(raw.canonical)
	if len(m.Blocks) == 0 {
		if raw.Content != nil {
			m.Blocks = append(m.Blocks, TextContentBlock(*raw.Content))
		}

		for _, call := range raw.Calls {
			m.Blocks = append(m.Blocks, ToolContentBlock(call))
		}
	}

	for i, block := range m.Blocks {
		if err := block.OK(); err != nil {
			return fmt.Errorf("invalid block %d: %w", i, err)
		}
	}

	return nil
}

type MessageOpt func(*Message) *Message

func WithID(id string) MessageOpt {
	return func(msg *Message) *Message {
		msg.ID = id
		return msg
	}
}

func WithRole(role Role) MessageOpt {
	return func(msg *Message) *Message {
		msg.Role = role
		return msg
	}
}

func WithTextContent(content string) MessageOpt {
	return func(msg *Message) *Message {
		msg.Blocks.ReplaceType(BlockTypeText, Blocks{TextContentBlock(content)})
		return msg
	}
}

func WithImageContent(data []byte) MessageOpt {
	return WithImage(data, "image/png")
}

func WithImage(data []byte, mediaType string) MessageOpt {
	return func(msg *Message) *Message {
		msg.Blocks.ReplaceType(BlockTypeImage, Blocks{ImageContentBlock(data, mediaType)})
		return msg
	}
}

func WithToolCalls(calls ...ToolCall) MessageOpt {
	return func(msg *Message) *Message {
		replacements := make(Blocks, len(calls))
		for i, call := range calls {
			replacements[i] = ToolContentBlock(call)
		}

		msg.Blocks.ReplaceType(BlockTypeToolCall, replacements)

		return msg
	}
}

func WithError(err error) MessageOpt {
	return func(msg *Message) *Message {
		msg.Error = err.Error()
		return msg
	}
}

func WithLLMInfo(info *LLMInfo) MessageOpt {
	return func(msg *Message) *Message {
		msg.LLMInfo = info
		return msg
	}
}

func WithChunkContent(content string) MessageOpt {
	return func(msg *Message) *Message {
		for i := range slices.Backward(msg.Blocks) {
			if msg.Blocks[i].Type() == BlockTypeText {
				msg.Blocks[i].Text.Text += content

				return msg
			}
		}

		msg.Blocks = append(msg.Blocks, TextContentBlock(content))

		return msg
	}
}

func WithBlocks(blocks ...Block) MessageOpt {
	return func(msg *Message) *Message {
		msg.Blocks = Blocks(blocks).Clone()
		return msg
	}
}

func WithAppendedBlocks(blocks ...Block) MessageOpt {
	return func(m *Message) *Message {
		m.Blocks = append(m.Blocks, Blocks(blocks).Clone()...)
		return m
	}
}
