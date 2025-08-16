package llms

import (
	"log/slog"
	"strings"
	"time"

	"github.com/cneill/smoke/pkg/tools"
	"github.com/cneill/smoke/pkg/utils"
)

type Message struct {
	ID    string    `json:"id"`
	Added time.Time `json:"added"`

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
}

func NewMessage(opts ...MessageOpt) *Message {
	msg := &Message{
		ID:    utils.RandID(),
		Added: time.Now(),
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

	attrs = append(attrs, slog.String("content", m.Content))

	return slog.GroupValue(attrs...)
}

type MessageOpt func(message *Message) *Message

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
