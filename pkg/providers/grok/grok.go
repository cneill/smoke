package grok

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/cneill/smoke/pkg/llms"
	"github.com/cneill/smoke/pkg/providers/chatgpt"
	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
)

const (
	API_URL = "https://api.x.ai/v1" //nolint:revive
)

type Grok struct {
	config  *llms.Config
	logger  *slog.Logger
	chatgpt *chatgpt.ChatGPT
}

func configOK(config *llms.Config) error {
	if err := config.OK(); err != nil {
		return fmt.Errorf("base LLM config error: %w", err)
	}

	if config.Temperature < 0 || config.Temperature > 2 {
		return fmt.Errorf("Grok temperature must be between 0 and 2")
	}

	return nil
}

func New(config *llms.Config) (llms.LLM, error) {
	if err := configOK(config); err != nil {
		return nil, fmt.Errorf("error with Grok options: %w", err)
	}

	client := openai.NewClient(
		option.WithAPIKey(config.APIKey),
		option.WithBaseURL(API_URL),
	)

	wrapped := &chatgpt.ChatGPT{
		Config: config,
		Client: client,
	}

	grok := &Grok{
		config:  config,
		logger:  slog.Default().WithGroup(llms.LLMTypeGrok),
		chatgpt: wrapped,
	}

	return grok, nil
}

func (g *Grok) LLMInfo() *llms.LLMInfo {
	return &llms.LLMInfo{
		Type:      llms.LLMTypeGrok,
		ModelName: g.config.Model,
	}
}
func (g *Grok) RequiresSessionSystem() bool { return true }

func (g *Grok) StartConversation(ctx context.Context, session *llms.Session) llms.Conversation {
	return g.chatgpt.StartConversation(ctx, session)
}

// TODO: some way of controlling this for the wrapped ChatGPT?
// func (g *Grok) shouldStream() bool {
// 	return true
// }
