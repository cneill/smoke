// Package claude contains an implementation of [llms.LLM] for Anthropic's Claude.
package claude

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
	"github.com/cneill/smoke/pkg/llms"
)

type Claude struct {
	config *llms.Config
	logger *slog.Logger
	client anthropic.Client
}

func configOK(config *llms.Config) error {
	if err := config.OK(); err != nil {
		return fmt.Errorf("base LLM config error: %w", err)
	}

	if config.Temperature < 0 || config.Temperature > 1 {
		return fmt.Errorf("Claude temperature must be between 0 and 1")
	}

	return nil
}

func New(config *llms.Config) (llms.LLM, error) {
	if err := configOK(config); err != nil {
		return nil, fmt.Errorf("error with Claude options: %w", err)
	}

	client := anthropic.NewClient(
		option.WithAPIKey(config.APIKey),
	)

	claude := &Claude{
		config: config,
		logger: slog.Default().WithGroup(llms.LLMTypeClaude),
		client: client,
	}

	return claude, nil
}

func (c *Claude) LLMInfo() *llms.LLMInfo {
	return &llms.LLMInfo{
		Type:      llms.LLMTypeClaude,
		ModelName: c.config.Model,
	}
}

func (c *Claude) RequiresSessionSystem() bool { return false }

func (c *Claude) StartConversation(ctx context.Context, session *llms.Session) llms.Conversation {
	newCtx, cancel := context.WithCancelCause(ctx)

	conv := &conversation{
		id:           session.Name,
		stream:       c.shouldStream(),
		cancel:       cancel,
		eventChan:    make(chan llms.Event),
		continueChan: make(chan struct{}),
		session:      session, // TODO: read-only view
		llmInfo:      c.LLMInfo(),
		client:       c.client,
		config:       c.config,
	}

	go conv.run(newCtx)

	return conv
}

func (c *Claude) shouldStream() bool {
	return true
}
