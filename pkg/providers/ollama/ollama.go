// Package ollama contains an implementation of [llms.LLM] for locally-served models via Ollama.
// Ollama exposes an OpenAI-compatible API, so this provider reuses the ChatGPT conversation
// implementation with a custom base URL.
package ollama

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

type Ollama struct {
	config *llms.Config
	logger *slog.Logger
	client openai.Client
}

func configOK(config *llms.Config) error {
	if err := config.OK(); err != nil {
		return err //nolint:wrapcheck
	}

	if config.BaseURL == "" {
		return fmt.Errorf("missing base URL")
	}

	if config.Temperature < 0 || config.Temperature > 2 {
		return fmt.Errorf("Ollama temperature must be between 0 and 2")
	}

	return nil
}

func New(config *llms.Config) (llms.LLM, error) {
	if err := configOK(config); err != nil {
		return nil, fmt.Errorf("error with Ollama options: %w", err)
	}

	opts := []option.RequestOption{
		option.WithBaseURL(config.BaseURL),
	}

	if config.APIKey != "" {
		opts = append(opts, option.WithAPIKey(config.APIKey))
	} else {
		// The openai-go SDK requires a non-empty API key; Ollama does not validate it.
		opts = append(opts, option.WithAPIKey("ollama"))
	}

	client := openai.NewClient(opts...)

	ollama := &Ollama{
		config: config,
		logger: slog.Default().WithGroup(llms.LLMTypeOllama),
		client: client,
	}

	return ollama, nil
}

func (o *Ollama) LLMInfo() *llms.LLMInfo {
	return &llms.LLMInfo{
		Type:      llms.LLMTypeOllama,
		ModelName: o.config.Model,
	}
}

func (o *Ollama) RequiresSessionSystem() bool { return true }

func (o *Ollama) StartConversation(ctx context.Context, session *llms.Session) llms.Conversation {
	conv, newCtx, err := chatgpt.NewConversation(ctx, o.client, &base.ConversationOpts{
		Session: session,
		LLMInfo: o.LLMInfo(),
		Config:  o.config,
		Stream:  true,
	})
	if err != nil {
		// Config was already validated in New(), so this should never happen.
		panic(fmt.Sprintf("ollama: failed to create conversation: %v", err))
	}

	go conv.Start(newCtx)

	return conv
}
