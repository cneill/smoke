// Package claude contains an implementation of [llms.LLM] for Anthropic's Claude.
package claude

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
	"github.com/cneill/smoke/pkg/llms"
	"github.com/cneill/smoke/pkg/providers/base"
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
	// Two-step construction: the send funcs close over conv, so we build conv first, then wire the
	// base in.
	conv := &conversation{client: c.client}

	baseConv, newCtx, err := base.NewConversation(ctx, &base.ConversationOpts{
		Session:      session,
		LLMInfo:      c.LLMInfo(),
		Config:       c.config,
		Stream:       c.shouldStream(),
		SendStream:   conv.sendStream,
		SendNoStream: conv.sendNoStream,
	})
	if err != nil {
		// Config was already validated in New(), so this should never happen.
		panic(fmt.Sprintf("claude: failed to create base conversation: %v", err))
	}

	conv.Conversation = baseConv

	go conv.Start(newCtx)

	return conv
}

func (c *Claude) shouldStream() bool {
	return true
}
