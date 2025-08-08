package llms

import "context"

type LLMType string

const (
	LLMTypeChatGPT = "chatGPT"
	LLMTypeClaude  = "claude"
)

type LLM interface {
	Type() LLMType
	ModelName() string
	SendSession(ctx context.Context, s *Session) error
	RequiresSessionSystem() bool
}
