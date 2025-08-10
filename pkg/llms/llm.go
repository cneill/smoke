package llms

import "context"

type LLMType string

const (
	LLMTypeChatGPT = "chatGPT"
	LLMTypeClaude  = "claude"
)

type LLM interface {
	LLMInfo() LLMInfo
	SendSession(ctx context.Context, s *Session) (*Message, error)
	RequiresSessionSystem() bool
	HandleToolCalls(msg *Message) ([]*Message, error)
}

type LLMInfo struct {
	Type      LLMType
	ModelName string
}
