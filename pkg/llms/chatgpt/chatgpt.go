// Package chatgpt contains an implementation of [llms.LLM] for OpenAI's ChatGPT.
package chatgpt

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/cneill/smoke/pkg/llms"
	"github.com/cneill/smoke/pkg/tools"
	"github.com/openai/openai-go/v2"
	"github.com/openai/openai-go/v2/option"
	"github.com/openai/openai-go/v2/shared"
)

type Opts struct {
	APIKey       string
	Model        shared.ChatModel
	MaxTokens    int64
	ToolsManager *tools.Manager
}

func (o *Opts) OK() error {
	switch {
	case o.APIKey == "":
		return fmt.Errorf("missing api key")
	case o.Model == "":
		return fmt.Errorf("missing model")
	case o.MaxTokens <= 0:
		return fmt.Errorf("max tokens must be >0")
	case o.ToolsManager == nil:
		return fmt.Errorf("must supply a tools manager instance")
	}

	return nil
}

type ChatGPT struct {
	opts   *Opts
	logger *slog.Logger
	tools  *tools.Manager
	client openai.Client
}

func NewChatGPT(opts *Opts) (*ChatGPT, error) {
	if err := opts.OK(); err != nil {
		return nil, fmt.Errorf("error with ChatGPT options: %w", err)
	}

	client := openai.NewClient(
		option.WithAPIKey(opts.APIKey),
	)

	chatGPT := &ChatGPT{
		opts:   opts,
		logger: slog.Default().WithGroup("chatgpt"),
		tools:  opts.ToolsManager,
		client: client,
	}

	return chatGPT, nil
}

func (c *ChatGPT) LLMInfo() *llms.LLMInfo {
	return &llms.LLMInfo{
		Type:      llms.LLMTypeChatGPT,
		ModelName: c.opts.Model,
	}
}
func (c *ChatGPT) RequiresSessionSystem() bool { return true }

func (c *ChatGPT) newMessage(opts ...llms.MessageOpt) *llms.Message {
	msg := llms.NewMessage(
		llms.WithLLMInfo(c.LLMInfo()),
	)

	for _, opt := range opts {
		msg = opt(msg)
	}

	return msg
}

// getSessionMessages converts the generic messages in 'session' to messages appropriate for a ChatGPT conversation
// history.
func (c *ChatGPT) getSessionMessages(session *llms.Session) []openai.ChatCompletionMessageParamUnion {
	results := make([]openai.ChatCompletionMessageParamUnion, len(session.Messages))

	for num, msg := range session.Messages {
		switch msg.Role {
		case llms.RoleAssistant:
			assistantMsg := openai.AssistantMessage(msg.Content)

			if msg.HasToolCalls() {
				rawCalls, ok := msg.ToolCallInfo.([]openai.ChatCompletionMessageToolCallUnion)
				if ok {
					for _, toolCall := range rawCalls {
						assistantMsg.OfAssistant.ToolCalls = append(assistantMsg.OfAssistant.ToolCalls, toolCall.ToParam())
					}
				} else {
					c.logger.Warn("got ToolCallInfo of unexpected type", "type", fmt.Sprintf("%T", msg.ToolCallInfo))
				}
			}

			results[num] = assistantMsg
		case llms.RoleSystem:
			results[num] = openai.SystemMessage(msg.Content)
		case llms.RoleUser:
			results[num] = openai.UserMessage(msg.Content)
		case llms.RoleTool:
			results[num] = openai.ToolMessage(msg.Content, msg.ToolCallID)
		case llms.RoleUnknown:
			c.logger.Warn("got message with unknown role", "message", msg.Content)
		}
	}

	return results
}

func (c *ChatGPT) CompletionTools() []openai.ChatCompletionToolUnionParam {
	results := []openai.ChatCompletionToolUnionParam{}

	for _, tool := range c.tools.Tools {
		properties := map[string]any{}
		required := []string{}

		for _, param := range tool.Params() {
			keyParams := map[string]any{
				"type":        param.Type,
				"description": param.Description,
			}

			if param.Type == tools.ParamTypeArray {
				keyParams["items"] = map[string]any{
					"type": param.ItemType,
				}
			}

			properties[param.Key] = keyParams

			if param.Required {
				required = append(required, param.Key)
			}
		}

		params := openai.FunctionParameters{
			"type":       tools.ParamTypeObject,
			"properties": properties,
			"required":   required,
		}

		results = append(results, openai.ChatCompletionFunctionTool(openai.FunctionDefinitionParam{
			Name:        tool.Name(),
			Description: openai.String(tool.Description()),
			Parameters:  params,
		}))
	}

	return results
}

func (c *ChatGPT) SendSession(ctx context.Context, session *llms.Session) (*llms.Message, error) {
	options := openai.ChatCompletionNewParams{
		Messages: c.getSessionMessages(session),
		Model:    c.opts.Model,
		N:        openai.Int(1),
		Tools:    c.CompletionTools(),
	}

	latest := session.Last()
	if latest != nil {
		c.logger.Debug("sending session", "msg", latest)
	}

	result, err := c.client.Chat.Completions.New(ctx, options, option.WithMaxRetries(5))
	if err != nil {
		return nil, fmt.Errorf("%w: %w", llms.ErrCompletion, err)
	}

	c.logger.Debug("token usage", "prompt", result.Usage.PromptTokens, "completion", result.Usage.CompletionTokens)

	if len(result.Choices) == 0 {
		return nil, fmt.Errorf("%w: no messages returned", llms.ErrEmptyResponse)
	}

	if refusal := result.Choices[0].Message.Refusal; refusal != "" {
		return nil, fmt.Errorf("%w: %s", llms.ErrPromptRefused, refusal)
	}

	response := result.Choices[0].Message

	msg := c.newMessage(
		llms.WithRole(llms.RoleAssistant),
		llms.WithContent(response.Content),
		llms.WithToolsCalled(c.getToolCallNames(response.ToolCalls)...),
		llms.WithToolCallInfo(response.ToolCalls),
	)

	return msg, nil
}

func (c *ChatGPT) HandleToolCalls(msg *llms.Message) ([]*llms.Message, error) {
	if !msg.HasToolCalls() {
		return nil, llms.ErrNoToolCalls
	}

	toolCalls, ok := msg.ToolCallInfo.([]openai.ChatCompletionMessageToolCallUnion)
	if !ok {
		return nil, fmt.Errorf("tool call info was of unexpected type: %T", msg.ToolCallInfo)
	}

	results := make([]*llms.Message, len(toolCalls))

	for toolCallNum, toolCall := range toolCalls {
		name := toolCall.Function.Name

		var (
			content     string
			toolCallErr error
		)

		params, err := c.tools.Tools.Params(name)
		if err != nil {
			return nil, fmt.Errorf("failed to get params for tool %q: %w", name, err)
		}

		args, err := tools.GetArgs([]byte(toolCall.Function.Arguments), params)
		if err != nil {
			return nil, fmt.Errorf("failed to get args for tool %q: %w", name, err)
		}

		output, err := c.tools.CallTool(name, args)
		if err != nil {
			c.logger.Error("failed to call tool", "tool_name", name, "error", err)
			toolCallErr = fmt.Errorf("failed to call tool %q: %w", name, err)
			content = toolCallErr.Error()
		} else {
			content = output
		}

		toolCallResultMsg := c.newMessage(
			llms.WithRole(llms.RoleTool),
			llms.WithToolCallID(toolCall.ID),
			llms.WithToolCallArgs(args),
			llms.WithToolsCalled(toolCall.Function.Name),
			llms.WithContent(content),
			llms.WithError(err),
		)

		results[toolCallNum] = toolCallResultMsg
	}

	return results, nil
}

func (c *ChatGPT) getToolCallNames(toolCalls []openai.ChatCompletionMessageToolCallUnion) []string {
	results := []string{}
	for _, toolCall := range toolCalls {
		results = append(results, toolCall.Function.Name)
	}

	return results
}
