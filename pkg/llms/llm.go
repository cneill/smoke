// Package llms contains generalized functions and types for interacting with different LLM providers. The LLM interface
// is implemented by each provider to work with Sessions and Messages.
package llms

import (
	"context"
	"log/slog"
)

type LLMType string

const (
	LLMTypeChatGPT = "chatgpt"
	LLMTypeClaude  = "claude"
)

type LLM interface {
	LLMInfo() *LLMInfo
	SendSession(ctx context.Context, s *Session) (*Message, error)
	RequiresSessionSystem() bool
	HandleToolCalls(msg *Message, s *Session) ([]*Message, error)
}

type Initializer func(config *Config) (LLM, error)

type LLMInfo struct {
	Type      LLMType
	ModelName string
}

func (l *LLMInfo) LogValue() slog.Value {
	return slog.GroupValue(
		slog.String("type", string(l.Type)),
		slog.String("model_name", l.ModelName),
	)
}
