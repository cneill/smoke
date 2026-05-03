package grok

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/cneill/smoke/pkg/llms"
	"github.com/cneill/smoke/pkg/providers/base"
	"github.com/cneill/smoke/pkg/providers/chatgpt"
	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
)

const (
	API_URL = "https://api.x.ai/v1" //nolint:revive
)

type Grok struct {
	config *llms.Config
	logger *slog.Logger
	client openai.Client
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

	grok := &Grok{
		config: config,
		logger: slog.Default().WithGroup(llms.LLMTypeGrok),
		client: client,
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
	conv, newCtx, err := chatgpt.NewConversation(ctx, g.client, &base.ConversationOpts{
		Session: session,
		LLMInfo: g.LLMInfo(),
		Config:  g.config,
		Stream:  true,
	})
	if err != nil {
		// Config was already validated in New(), so this should never happen.
		panic(fmt.Sprintf("grok: failed to create conversation: %v", err))
	}

	go conv.Start(newCtx)

	return conv
}
