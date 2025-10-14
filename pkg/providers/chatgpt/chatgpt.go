// Package chatgpt contains an implementation of [llms.LLM] for OpenAI's ChatGPT.
package chatgpt

import (
	"context"
	"fmt"
	"strings"

	"github.com/cneill/smoke/pkg/llms"
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

func (c *ChatGPT) StartConversation(ctx context.Context, session *llms.Session) llms.Conversation {
	newCtx, cancel := context.WithCancelCause(ctx)

	conv := &conversation{
		id:           session.Name,
		stream:       c.shouldStream(),
		cancel:       cancel,
		eventChan:    make(chan llms.Event),
		continueChan: make(chan struct{}),
		session:      session, // TODO: read-only view
		llmInfo:      c.LLMInfo(),
		client:       c.Client,
		config:       c.Config,
	}

	go conv.run(newCtx)

	return conv
}

func (c *ChatGPT) shouldStream() bool {
	// GPT-5 requires photo ID verification for streaming...
	return !strings.Contains(c.Config.Model, "gpt-5")
}
