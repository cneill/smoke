package llms

import (
	"fmt"
	"log/slog"
	"math/rand/v2"
	"strings"
	"time"
)

type Message struct {
	ID      string    `json:"id"`
	Added   time.Time `json:"added"`
	Updated time.Time `json:"updated"`

	Role    Role   `json:"role"`
	Content string `json:"content,omitempty"`
	Error   string `json:"error,omitempty"`

	// ToolCalls holds all tool calls made by the provider in Assistant messages and the details of the (one) original
	// Assistant call for Tool messages.
	ToolCalls ToolCalls `json:"tool_calls,omitempty"`

	// LLMInfo contains details about the LLM that generated the assistant message
	LLMInfo *LLMInfo `json:"llm_info,omitempty"`
}

func NewMessage(opts ...MessageOpt) *Message {
	now := time.Now()

	msg := &Message{
		ID:      randID(),
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
		WithContent(content),
	)
}

func (m *Message) OK() error {
	switch {
	case m.ID == "":
		return fmt.Errorf("message is missing ID")
	case m.Role == "":
		return fmt.Errorf("message is missing role")
	case m.Role == RoleTool && len(m.ToolCalls) == 0:
		return fmt.Errorf("message with %q role is missing tool call information", RoleTool)
	}

	return nil
}

func (m *Message) Clone() *Message {
	newMessage := Message{
		ID:        m.ID,
		Added:     m.Added,
		Updated:   m.Updated,
		Role:      m.Role,
		Content:   m.Content,
		Error:     m.Error,
		ToolCalls: m.ToolCalls.Clone(),
	}

	if m.LLMInfo != nil {
		temp := *m.LLMInfo
		newMessage.LLMInfo = &temp
	}

	return &newMessage
}

func (m *Message) Update(opts ...MessageOpt) *Message {
	clone := m.Clone()

	for _, opt := range opts {
		clone = opt(clone)
	}

	clone.Updated = time.Now()

	return clone
}

func (m *Message) HasToolCalls() bool {
	return len(m.ToolCalls) > 0
}

func (m *Message) LogValue() slog.Value {
	attrs := []slog.Attr{
		slog.String("id", m.ID),
		slog.String("role", string(m.Role)),
		slog.Time("added", m.Added),
		slog.Bool("has_tool_calls", m.HasToolCalls()),
	}

	if m.HasToolCalls() {
		attrs = append(attrs, slog.Any("tool_calls", m.ToolCalls))
	}

	if m.Error != "" {
		attrs = append(attrs, slog.String("error", m.Error))
	}

	if m.LLMInfo != nil {
		attrs = append(attrs, slog.Any("llm_info", m.LLMInfo))
	}

	attrs = append(attrs, slog.String("content", m.Content))

	return slog.GroupValue(attrs...)
}

func (m *Message) ToMarkdown() string {
	builder := &strings.Builder{}

	// header
	fmt.Fprintf(builder, "# %s\n*(%s)*\n\n", m.Role, m.Added.Format(time.RFC1123))

	// print the details of each tool call, if any
	if m.HasToolCalls() {
		for _, toolCall := range m.ToolCalls {
			fmt.Fprintf(builder, "**Tool called:** `%s`\n", toolCall.Name)
			fmt.Fprintf(builder, "**Args:** `%s`\n", toolCall.Args.String())
		}

		if m.Content != "" {
			fmt.Fprintf(builder, "**Content:**\n\n%s\n", m.Content)
		}
	} else {
		builder.WriteString(m.Content)
	}

	// footer
	builder.WriteString("\n\n----\n")

	return builder.String()
}

type MessageOpt func(message *Message) *Message

func WithID(id string) MessageOpt {
	return func(message *Message) *Message {
		message.ID = id
		return message
	}
}

func WithContent(content string) MessageOpt {
	return func(message *Message) *Message {
		message.Content = content
		return message
	}
}

func WithRole(role Role) MessageOpt {
	return func(message *Message) *Message {
		message.Role = role
		return message
	}
}

func WithToolCalls(toolCalls ...ToolCall) MessageOpt {
	return func(message *Message) *Message {
		message.ToolCalls = toolCalls
		return message
	}
}

func WithError(err error) MessageOpt {
	return func(message *Message) *Message {
		message.Error = err.Error()
		return message
	}
}

func WithLLMInfo(info *LLMInfo) MessageOpt {
	return func(message *Message) *Message {
		message.LLMInfo = info
		return message
	}
}

func WithChunkContent(content string) MessageOpt {
	return func(message *Message) *Message {
		// TODO: mutex?
		message.Content += content
		return message
	}
}

const idChars = "abcdef0123456789"

// randID returns a random 16-character hex string
func randID() string {
	output := []byte{}
	for range 16 {
		output = append(output, idChars[rand.IntN(len(idChars))]) //nolint:gosec
	}

	return string(output)
}
