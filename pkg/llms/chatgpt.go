package llms

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/cneill/smoke/pkg/tools"
	"github.com/openai/openai-go/v2"
	"github.com/openai/openai-go/v2/option"
	"github.com/openai/openai-go/v2/shared"
)

type ChatGPTOpts struct {
	APIKey       string
	Model        shared.ChatModel
	MaxTokens    int64
	ToolsManager *tools.Manager
}

func (c *ChatGPTOpts) OK() error {
	switch {
	case c.APIKey == "":
		return fmt.Errorf("missing api key")
	case c.Model == "":
		return fmt.Errorf("missing model")
	case c.MaxTokens <= 0:
		return fmt.Errorf("max tokens must be >0")
	case c.ToolsManager == nil:
		return fmt.Errorf("must supply a tools manager instance")
	}

	return nil
}

type ChatGPT struct {
	opts   *ChatGPTOpts
	logger *slog.Logger
	tools  *tools.Manager
	client openai.Client
}

func NewChatGPT(opts *ChatGPTOpts) (*ChatGPT, error) {
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

func (c *ChatGPT) Type() LLMType               { return LLMTypeChatGPT }
func (c *ChatGPT) ModelName() string           { return c.opts.Model }
func (c *ChatGPT) RequiresSessionSystem() bool { return true }

// getSessionMessages converts the generic messages in 'session' to messages appropriate for a ChatGPT conversation
// history.
func (c *ChatGPT) getSessionMessages(session *Session) []openai.ChatCompletionMessageParamUnion {
	results := make([]openai.ChatCompletionMessageParamUnion, len(session.Messages))

	for num, msg := range session.Messages {
		switch msg.Role {
		case RoleAssistant:
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
		case RoleSystem:
			results[num] = openai.SystemMessage(msg.Content)
		case RoleUser:
			results[num] = openai.UserMessage(msg.Content)
		case RoleTool:
			results[num] = openai.ToolMessage(msg.Content, msg.ToolCallID)
		case RoleUnknown:
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
			properties[param.Key] = map[string]any{
				"type":        param.Type,
				"description": param.Description,
			}

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

func (c *ChatGPT) SendSession(ctx context.Context, session *Session) error {
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

	result, err := c.client.Chat.Completions.New(ctx, options)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrCompletion, err)
	}

	if len(result.Choices) == 0 {
		return fmt.Errorf("%w: no messages returned", ErrEmptyResponse)
	}

	if refusal := result.Choices[0].Message.Refusal; refusal != "" {
		return fmt.Errorf("%w: %s", ErrPromptRefused, refusal)
	}

	if err := c.handleToolCalls(ctx, session, result.Choices[0].Message); err != nil {
		return fmt.Errorf("failed to handle tool calls: %w", err)
	}

	return nil
}

func (c *ChatGPT) handleToolCalls(ctx context.Context, session *Session, message openai.ChatCompletionMessage) error {
	toolCalls := message.ToolCalls

	msg := NewMessage(
		WithRole(RoleAssistant),
		WithContent(message.Content),
		WithToolsCalled(c.getToolCallNames(toolCalls)...),
		WithToolCallInfo(toolCalls),
	)

	session.AddMessage(msg)

	for _, toolCall := range toolCalls {
		// TODO: refactor to return a *Message from CallTool()?
		name := toolCall.Function.Name

		var (
			content     string
			toolCallErr error
		)

		params, err := c.tools.Tools.Params(name)
		if err != nil {
			return fmt.Errorf("failed to get params for tool %q: %w", name, err)
		}

		args, err := tools.GetArgs([]byte(toolCall.Function.Arguments), params)
		if err != nil {
			return fmt.Errorf("failed to get args for tool %q: %w", name, err)
		}

		output, err := c.tools.CallTool(name, args)
		if err != nil {
			c.logger.Error("failed to call tool", "tool_name", name, "error", err)
			toolCallErr = fmt.Errorf("failed to call tool %q: %w", name, err)
			content = toolCallErr.Error()
		} else {
			content = output
		}

		toolCallMsg := NewMessage(
			WithRole(RoleTool),
			WithToolCallID(toolCall.ID),
			WithToolCallArgs(args),
			WithContent(content),
			WithError(err),
		)

		session.AddMessage(toolCallMsg)
	}

	if msg.HasToolCalls() {
		if err := c.SendSession(ctx, session); err != nil {
			return fmt.Errorf("failed to send session with tool call results: %w", err)
		}
	}

	return nil
}

func (c *ChatGPT) getToolCallNames(toolCalls []openai.ChatCompletionMessageToolCallUnion) []string {
	results := []string{}
	for _, toolCall := range toolCalls {
		results = append(results, toolCall.Function.Name)
	}

	return results
}
