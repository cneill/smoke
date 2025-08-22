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
)

type ChatGPT struct {
	config *llms.Config
	logger *slog.Logger
	client openai.Client
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
		config: config,
		logger: slog.Default().WithGroup(llms.LLMTypeChatGPT),
		client: client,
	}

	return chatGPT, nil
}

func (c *ChatGPT) LLMInfo() *llms.LLMInfo {
	return &llms.LLMInfo{
		Type:      llms.LLMTypeChatGPT,
		ModelName: c.config.Model,
	}
}
func (c *ChatGPT) RequiresSessionSystem() bool { return true }

func (c *ChatGPT) SendSession(ctx context.Context, session *llms.Session) (*llms.Message, error) {
	options := openai.ChatCompletionNewParams{
		MaxCompletionTokens: openai.Int(c.config.MaxTokens),
		Messages:            c.getSessionMessages(session),
		Model:               c.config.Model,
		N:                   openai.Int(1),
		Tools:               c.completionTools(session),
		Temperature:         openai.Float(c.config.Temperature),
	}

	latest := session.Last()
	if latest != nil {
		c.logger.Debug("sending session", "msg", latest)
	}

	result, err := c.client.Chat.Completions.New(ctx, options, option.WithMaxRetries(5))
	if err != nil {
		return nil, fmt.Errorf("%w: %w", llms.ErrCompletion, err)
	}

	session.UpdateUsage(result.Usage.PromptTokens, result.Usage.CompletionTokens)

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

func (c *ChatGPT) HandleToolCalls(msg *llms.Message, session *llms.Session) ([]*llms.Message, error) {
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

		params, err := session.Tools.Params(name)
		if err != nil {
			return nil, fmt.Errorf("failed to get params for tool %q: %w", name, err)
		}

		args, err := tools.GetArgs([]byte(toolCall.Function.Arguments), params)
		if err != nil {
			return nil, fmt.Errorf("failed to get args for tool %q: %w", name, err)
		}

		output, err := session.Tools.CallTool(context.TODO(), name, args)
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

func (c *ChatGPT) completionTools(session *llms.Session) []openai.ChatCompletionToolUnionParam {
	results := []openai.ChatCompletionToolUnionParam{}

	for _, tool := range session.Tools.GetTools() {
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

func (c *ChatGPT) getToolCallNames(toolCalls []openai.ChatCompletionMessageToolCallUnion) []string {
	results := []string{}
	for _, toolCall := range toolCalls {
		results = append(results, toolCall.Function.Name)
	}

	return results
}

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
