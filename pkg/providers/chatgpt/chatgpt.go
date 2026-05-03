// Package chatgpt contains an implementation of [llms.LLM] for OpenAI's ChatGPT.
package chatgpt

import (
	"context"
	"fmt"

	"github.com/cneill/smoke/pkg/llms"
	"github.com/cneill/smoke/pkg/providers/base"
	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
)

type ChatGPT struct {
	Config *llms.Config
	Client openai.Client
}

func configOK(config *llms.Config) error {
	if err := config.OK(); err != nil {
		return fmt.Errorf("base LLM config error: %w", err)
	}

	if config.Temperature < 0 || config.Temperature > 2 {
		return fmt.Errorf("ChatGPT temperature must be between 0 and 2")
	}

	return nil
}

func New(config *llms.Config) (llms.LLM, error) {
	if err := configOK(config); err != nil {
		return nil, fmt.Errorf("error with ChatGPT options: %w", err)
	}

	client := openai.NewClient(
		option.WithAPIKey(config.APIKey),
	)

	chatGPT := &ChatGPT{
		Config: config,
		Client: client,
	}

	return chatGPT, nil
}

func (c *ChatGPT) LLMInfo() *llms.LLMInfo {
	return &llms.LLMInfo{
		Type:      llms.LLMTypeChatGPT,
		ModelName: c.Config.Model,
	}
}
func (c *ChatGPT) RequiresSessionSystem() bool { return true }

// NewConversation constructs a chatgpt conversation using the provided base opts. This allows
// callers (e.g. the grok provider) to reuse the ChatGPT wire protocol while supplying their own
// LLMInfo, config, or stream preference. The opts.SendStream and opts.SendNoStream fields are
// set automatically and must be left nil by the caller.
//
// The caller is responsible for launching the conversation via go conv.Start(newCtx).
func NewConversation(ctx context.Context, client openai.Client, opts *base.ConversationOpts) (llms.Conversation, context.Context, error) {
	// Two-step construction: the send funcs close over conv, so we build conv first and set the
	// funcs before calling base.NewConversation.
	conv := &conversation{client: client}
	opts.SendStream = conv.sendStream
	opts.SendNoStream = conv.sendNoStream

	baseConv, newCtx, err := base.NewConversation(ctx, opts)
	if err != nil {
		return nil, nil, fmt.Errorf("chatgpt: failed to create base conversation: %w", err)
	}

	conv.Conversation = baseConv

	return conv, newCtx, nil
}

func (c *ChatGPT) StartConversation(ctx context.Context, session *llms.Session) llms.Conversation {
	conv, newCtx, err := NewConversation(ctx, c.Client, &base.ConversationOpts{
		Session: session,
		LLMInfo: c.LLMInfo(),
		Config:  c.Config,
		Stream:  true,
	})
	if err != nil {
		// Config was already validated in New(), so this should never happen.
		panic(fmt.Sprintf("chatgpt: failed to create conversation: %v", err))
	}

	go conv.Start(newCtx)

	return conv
}
