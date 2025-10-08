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

type conversation struct {
	id           string
	ctx          context.Context // TODO: this is frowned-upon... accept and return contexts?
	cancel       context.CancelFunc
	eventChan    chan llms.Event
	continueChan chan struct{}
	session      *llms.Session // TODO: read-only snapshot of Session as provided by Smoke
	client       openai.Client
	config       *llms.Config
}

func (c *conversation) llmInfo() *llms.LLMInfo {
	return &llms.LLMInfo{
		Type:      llms.LLMTypeChatGPT,
		ModelName: c.config.Model,
	}
}

func (c *conversation) ID() string { return c.id }

func (c *conversation) Events() <-chan llms.Event {
	return c.eventChan
}

func (c *conversation) Cancel() {
	c.cancel()
}

func (c *conversation) Continue(ctx context.Context) error {
	select {
	case c.continueChan <- struct{}{}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	case <-c.ctx.Done():
		return c.ctx.Err()
	}
}

func (c *conversation) Close() error {
	c.cancel()
	return nil
}

func (c *conversation) run() {
	defer close(c.eventChan)

	// do the thing

	select {
	case c.eventChan <- llms.EventDone{}:
	case <-c.ctx.Done():
	}
}

func (c *conversation) send() error {
	options := openai.ChatCompletionNewParams{
		MaxCompletionTokens: openai.Int(c.config.MaxTokens),
		Messages:            c.getSessionMessages(c.session),
		Model:               c.config.Model,
		N:                   openai.Int(1),
		Tools:               c.completionTools(c.session),
		Temperature:         openai.Float(c.config.Temperature),
	}

	result, err := c.client.Chat.Completions.New(c.ctx, options, option.WithMaxRetries(5))
	if err != nil {
		return fmt.Errorf("%w: %w", llms.ErrCompletion, err)
	}

	c.emit(llms.EventUsageUpdate{
		InputTokens:  result.Usage.PromptTokens,
		OutputTokens: result.Usage.CompletionTokens,
	})

	slog.Debug("token usage", "prompt", result.Usage.PromptTokens, "completion", result.Usage.CompletionTokens)

	if len(result.Choices) == 0 {
		return fmt.Errorf("%w: no messages returned", llms.ErrEmptyResponse)
	}

	if refusal := result.Choices[0].Message.Refusal; refusal != "" {
		return fmt.Errorf("%w: %s", llms.ErrPromptRefused, refusal)
	}

	response := result.Choices[0].Message

	if response.ToParam().OfAssistant == nil {
		return fmt.Errorf("%w: no assistant message", llms.ErrEmptyResponse)
	}

	// We convert this ToParam() because it's the only way to get streaming responses to work...
	// TODO: need this here?
	toolCalls, err := c.vendorToolCallsToGeneric(response.ToParam().OfAssistant.ToolCalls...)
	if err != nil {
		return fmt.Errorf("failed to handle assistant tool calls: %w", err)
	}

	if len(toolCalls) > 0 {
		c.emit(llms.EventToolCallsRequested{
			Calls: toolCalls,
		})

		if err := c.waitForContinue(); err != nil {
			return fmt.Errorf("error while waiting: %w", err)
		}
	} else {
		c.emit(llms.EventFinalMessage{
			Message: c.newMessage(
				llms.WithID(result.ID),
				llms.WithRole(llms.RoleAssistant),
				llms.WithContent(response.Content),
			),
		})
	}

	return nil
}

func (c *conversation) newMessage(opts ...llms.MessageOpt) *llms.Message {
	msg := llms.NewMessage(
		llms.WithLLMInfo(c.llmInfo()),
	)

	for _, opt := range opts {
		msg = opt(msg)
	}

	return msg
}

// getSessionMessages converts the generic messages in 'session' to messages appropriate for a ChatGPT conversation
// history.
func (c *conversation) getSessionMessages(session *llms.Session) []openai.ChatCompletionMessageParamUnion {
	results := make([]openai.ChatCompletionMessageParamUnion, len(session.Messages))

	for num, msg := range session.Messages {
		switch msg.Role {
		case llms.RoleAssistant:
			assistantMsg := openai.AssistantMessage(msg.Content)

			if msg.HasToolCalls() {
				assistantMsg.OfAssistant.ToolCalls = c.genericToolCallsToVendor(msg.ToolCalls...)
			}

			results[num] = assistantMsg
		case llms.RoleSystem:
			results[num] = openai.SystemMessage(msg.Content)
		case llms.RoleUser:
			results[num] = openai.UserMessage(msg.Content)
		case llms.RoleTool:
			if n := len(msg.ToolCalls); n > 1 {
				slog.Warn("more than one tool call referenced in message with tool role; skipping", "num", n, "names", msg.ToolCalls.Names())
				continue
			}

			results[num] = openai.ToolMessage(msg.Content, msg.ToolCalls[0].ID)
		case llms.RoleUnknown:
			slog.Warn("got message with unknown role", "message", msg.Content)
		}
	}

	return results
}

func (c *conversation) vendorToolCallsToGeneric(toolCalls ...openai.ChatCompletionMessageToolCallUnionParam) (llms.ToolCalls, error) {
	results := make(llms.ToolCalls, len(toolCalls))

	for i, toolCall := range toolCalls {
		if toolCall.OfFunction == nil {
			tcType := toolCall.GetType()
			return nil, fmt.Errorf("got a tool call of type other than function: %s", *tcType)
		}

		name := toolCall.OfFunction.Function.Name
		args, err := c.session.Tools.GetArgs(name, []byte(toolCall.OfFunction.Function.Arguments))
		if err != nil {
			return nil, fmt.Errorf("failed to parse arguments for tool call to tool %q: %w", name, err)
		}

		results[i] = llms.ToolCall{
			ID:   toolCall.OfFunction.ID,
			Name: name,
			Args: args,
		}
	}

	return results, nil
}

func (c *conversation) genericToolCallsToVendor(toolCalls ...llms.ToolCall) []openai.ChatCompletionMessageToolCallUnionParam {
	results := make([]openai.ChatCompletionMessageToolCallUnionParam, len(toolCalls))

	for i, toolCall := range toolCalls {
		results[i] = openai.ChatCompletionMessageToolCallUnionParam{
			OfFunction: &openai.ChatCompletionMessageFunctionToolCallParam{
				ID: toolCall.ID,
				Function: openai.ChatCompletionMessageFunctionToolCallFunctionParam{
					Arguments: toolCall.Args.String(),
					Name:      toolCall.Name,
				},
				// Type: constant.Function(),
			},
		}
	}

	return results
}

func (c *conversation) completionTools(session *llms.Session) []openai.ChatCompletionToolUnionParam {
	results := []openai.ChatCompletionToolUnionParam{}

	for _, tool := range session.Tools.GetTools() {
		params := tool.Params()

		properties, err := params.JSONSchemaProperties()
		if err != nil {
			slog.Error("failed to get JSON Schema properties for tool, skipping", "tool_name", tool.Name(), "error", err)
			continue
		}

		toolDef := openai.ChatCompletionFunctionTool(openai.FunctionDefinitionParam{
			Name:        tool.Name(),
			Description: openai.String(tool.Description()),
			Parameters: openai.FunctionParameters{
				"type":       tools.ParamTypeObject,
				"properties": properties,
				"required":   params.RequiredKeys(),
			},
		})
		results = append(results, toolDef)
	}

	return results
}

func (c *conversation) waitForContinue() error {
	select {
	case <-c.continueChan:
		return nil
	case <-c.ctx.Done():
		return c.ctx.Err()
	}
}

func (c *conversation) emit(e llms.Event) bool {
	select {
	case c.eventChan <- e:
		return true
	case <-c.ctx.Done():
		return false
	}
}
