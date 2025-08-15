// Package llms contains generalized functions and types for interacting with different LLM providers. The LLM interface
// is implemented by each provider to work with Sessions and Messages.
package llms

import "context"

type LLMType string

const (
	LLMTypeChatGPT = "chatgpt"
	LLMTypeClaude  = "claude"
)

type LLM interface {
	LLMInfo() *LLMInfo
	SendSession(ctx context.Context, s *Session) (*Message, error)
	RequiresSessionSystem() bool
	HandleToolCalls(msg *Message) ([]*Message, error)
}

type LLMInfo struct {
	Type      LLMType
	ModelName string
}
