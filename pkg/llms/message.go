package llms

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/cneill/smoke/pkg/tools"
	"github.com/cneill/smoke/pkg/utils"
)

type Message struct {
	ID      string    `json:"id"`
	Added   time.Time `json:"added"`
	Updated time.Time `json:"updated"`

	Role    Role   `json:"role"`
	Content string `json:"content,omitempty"`
	Error   error  `json:"error,omitempty"`

	// ToolsCalled contains the names of tools the assistant requested to use.
	ToolsCalled []string `json:"tools_called,omitempty"`
	// ToolCallInfo contains the raw representation of the tool call information from the assistant.
	// TODO: hide this?
	ToolCallInfo any `json:"tool_call_info,omitempty,omitzero"`

	// ToolCallID is the ID associated with the assistant's tool use request.
	ToolCallID string `json:"tool_call_id,omitempty"` // TODO: Should this be a []string?
	// ToolCallArgs are the arguments provided by the assistant to the specified tool.
	ToolCallArgs tools.Args `json:"tool_call_args,omitempty"`

	// LLMInfo contains details about the LLM that generated the assistant message
	LLMInfo *LLMInfo `json:"llm_info,omitempty"`

	// IsStreamed tells us whether this response was streamed from the LLM provider. Defaults to false.
	IsStreamed bool
	// IsInitial signals that this is the FIRST streamed message which will be updated subsequently.
	IsInitial bool
	// IsChunk tells us whether this message is a full one or just a chunk that has been streamed from the provider.
	IsChunk bool
	// IsFinalized tells us whether this streamed message has all its chunks.
	IsFinalized bool
}

func NewMessage(opts ...MessageOpt) *Message {
	now := time.Now()

	msg := &Message{
		ID:      utils.RandID(),
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

func ChunkMessage(role Role, id, content string) *Message {
	return NewMessage(
		WithID(id),
		WithRole(role),
		WithContent(content),
		WithIsStreamed(true),
		WithIsInitial(true),
		WithIsChunk(true),
	)
}

func (m *Message) Update(opts ...MessageOpt) *Message {
	for _, opt := range opts {
		m = opt(m)
	}

	m.Updated = time.Now()

	return m
}

// TODO: add OK() method

func (m *Message) HasToolCalls() bool { return len(m.ToolsCalled) > 0 }

func (m *Message) LogValue() slog.Value {
	attrs := []slog.Attr{
		slog.String("id", m.ID),
		slog.String("role", string(m.Role)),
		slog.Time("added", m.Added),
		slog.Bool("has_tool_calls", m.HasToolCalls()),
	}

	if m.HasToolCalls() {
		toolCallAttrs := []slog.Attr{
			slog.String("tools_called", strings.Join(m.ToolsCalled, ",")),
			slog.String("tool_call_id", m.ToolCallID),
		}

		if m.ToolCallInfo != nil {
			if tcBytes, err := json.Marshal(m.ToolCallInfo); err != nil {
				panic(err)
			} else {
				slog.Debug("we marshalled the call info for logging...", "output", string(tcBytes))
			}

			toolCallAttrs = append(toolCallAttrs, slog.Any("call_info", m.ToolCallInfo))
		}

		if m.ToolCallArgs != nil {
			toolCallAttrs = append(toolCallAttrs, slog.Any("args", m.ToolCallArgs))
		}

		attrs = append(attrs, slog.GroupAttrs("tool_calls", toolCallAttrs...))
	}

	if m.Error != nil {
		attrs = append(attrs, slog.String("error", m.Error.Error()))
	}

	if m.LLMInfo != nil {
		attrs = append(attrs, slog.Any("llm_info", m.LLMInfo))
	}

	if m.IsStreamed {
		streamAttrs := []slog.Attr{
			slog.Bool("is_initial", m.IsInitial),
			slog.Bool("is_chunk", m.IsChunk),
			slog.Bool("is_finalized", m.IsFinalized),
		}

		attrs = append(attrs, slog.GroupAttrs("streaming", streamAttrs...))
	}

	attrs = append(attrs, slog.String("content", m.Content))

	return slog.GroupValue(attrs...)
}

func (m *Message) ToMarkdown() string {
	header := fmt.Sprintf("# %s\n*(%s)*\n\n", m.Role, m.Added.Format(time.RFC1123))
	footer := "\n\n----\n"

	var body string

	if !m.HasToolCalls() {
		body = m.Content
	} else {
		body = fmt.Sprintf("**Tools called:** `%s`\n", strings.Join(m.ToolsCalled, ", "))
		if m.ToolCallArgs != nil {
			body += fmt.Sprintf("**Tool call args:** `%s`\n", m.ToolCallArgs.String())
		}

		if m.Content != "" {
			body += fmt.Sprintf("**Content:**\n\n%s\n", m.Content)
		}
	}

	return header + body + footer
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

func WithToolsCalled(toolNames ...string) MessageOpt {
	return func(message *Message) *Message {
		message.ToolsCalled = toolNames
		return message
	}
}

func WithToolCallInfo(toolCallInfo any) MessageOpt {
	return func(message *Message) *Message {
		message.ToolCallInfo = toolCallInfo
		return message
	}
}

func WithToolCallID(toolCallID string) MessageOpt {
	return func(message *Message) *Message {
		message.ToolCallID = toolCallID
		return message
	}
}

func WithToolCallArgs(args tools.Args) MessageOpt {
	return func(message *Message) *Message {
		message.ToolCallArgs = args
		return message
	}
}

func WithError(err error) MessageOpt {
	return func(message *Message) *Message {
		message.Error = err
		return message
	}
}

func WithLLMInfo(info *LLMInfo) MessageOpt {
	return func(message *Message) *Message {
		message.LLMInfo = info
		return message
	}
}

func WithIsStreamed(isStreamed bool) MessageOpt {
	return func(message *Message) *Message {
		message.IsStreamed = isStreamed
		return message
	}
}

func WithIsInitial(isInitial bool) MessageOpt {
	return func(message *Message) *Message {
		message.IsInitial = isInitial
		return message
	}
}

func WithIsChunk(isChunk bool) MessageOpt {
	return func(message *Message) *Message {
		message.IsChunk = isChunk
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

func WithIsFinalized(isFinalized bool) MessageOpt {
	return func(message *Message) *Message {
		message.IsFinalized = isFinalized
		return message
	}
}
