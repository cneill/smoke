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

const maxIterations = 1024

type conversation struct {
	id           string
	stream       bool
	cancel       context.CancelCauseFunc
	eventChan    chan llms.Event
	continueChan chan struct{}
	session      *llms.Session // TODO: read-only snapshot of Session as provided by Smoke
	client       openai.Client
	config       *llms.Config

	hasPendingToolCalls bool
}

func (c *conversation) ID() string { return c.id }

func (c *conversation) Events() <-chan llms.Event {
	return c.eventChan
}

func (c *conversation) Cancel(err error) {
	c.cancel(err)
}

func (c *conversation) Continue(ctx context.Context) error {
	select {
	case c.continueChan <- struct{}{}:
		return nil
	case <-ctx.Done():
		return fmt.Errorf("conversation context error: %w", ctx.Err())
	}
}

func (c *conversation) Close() {
	c.cancel(nil)
}

func (c *conversation) llmInfo() *llms.LLMInfo {
	return &llms.LLMInfo{
		Type:      llms.LLMTypeChatGPT,
		ModelName: c.config.Model,
	}
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

func (c *conversation) run(ctx context.Context) {
	defer close(c.eventChan)

	for range maxIterations {
		if c.stream {
			if err := c.sendStream(ctx); err != nil {
				c.eventChan <- llms.EventError{
					Err: fmt.Errorf("failed to send message (streaming): %w", err),
				}
			}
		} else {
			if err := c.sendNoStream(ctx); err != nil {
				c.eventChan <- llms.EventError{
					Err: fmt.Errorf("failed to send message (non-streaming): %w", err),
				}
			}
		}

		if !c.hasPendingToolCalls {
			break
		}

		if err := c.waitForContinue(ctx); err != nil {
			c.eventChan <- llms.EventError{
				Err: fmt.Errorf("failed while waiting for tool call results: %w", err),
			}
		}
	}

	select {
	case c.eventChan <- llms.EventDone{}:
	case <-ctx.Done():
		slog.Debug("context cancelled before sending done event", "error", ctx.Err())
	}
}

func (c *conversation) sendNoStream(ctx context.Context) error {
	options := c.getNewCompletionParams()

	result, err := c.client.Chat.Completions.New(ctx, options, option.WithMaxRetries(5))
	if err != nil {
		return fmt.Errorf("%w: %w", llms.ErrCompletion, err)
	}

	c.emit(ctx, llms.EventUsageUpdate{
		InputTokens:  result.Usage.PromptTokens,
		OutputTokens: result.Usage.CompletionTokens,
	})

	if len(result.Choices) == 0 {
		return fmt.Errorf("%w: no messages returned", llms.ErrEmptyResponse)
	}

	if refusal := result.Choices[0].Message.Refusal; refusal != "" {
		return fmt.Errorf("%w: %s", llms.ErrPromptRefused, refusal)
	}

	response := result.Choices[0].Message

	if err := c.handleResponse(ctx, result.ID, response); err != nil {
		return err
	}

	return nil
}

func (c *conversation) sendStream(ctx context.Context) error {
	options := c.getNewCompletionParams()

	stream := c.client.Chat.Completions.NewStreaming(ctx, options, option.WithMaxRetries(5))
	defer stream.Close()

	accumulator := openai.ChatCompletionAccumulator{}

	for stream.Next() {
		chunk := stream.Current()
		accumulator.AddChunk(chunk)

		if _, ok := accumulator.JustFinishedContent(); ok {
			slog.Debug("got end of content",
				"current_delta", chunk.Choices[0].Delta.Content,
				"finish_reason", chunk.Choices[0].FinishReason)
		}

		if _, ok := accumulator.JustFinishedToolCall(); ok {
			slog.Debug("got end of tool call info",
				"current_delta", chunk.Choices[0].Delta.Content,
				"tool_calls", accumulator.Choices[0].Message.ToolCalls)
		}

		if refusal, ok := accumulator.JustFinishedRefusal(); ok {
			return fmt.Errorf("%w: %s", llms.ErrPromptRefused, refusal)
		}

		c.emit(ctx, llms.EventTextDelta{
			Text: chunk.Choices[0].Delta.Content,
		})
	}

	if err := stream.Err(); err != nil {
		return fmt.Errorf("%w: streaming: %w", llms.ErrCompletion, err)
	}

	c.emit(ctx, llms.EventUsageUpdate{
		InputTokens:  accumulator.Usage.PromptTokens,
		OutputTokens: accumulator.Usage.CompletionTokens,
	})

	if len(accumulator.Choices) == 0 {
		return fmt.Errorf("%w: no messages returned", llms.ErrEmptyResponse)
	}

	response := accumulator.Choices[0].Message

	if err := c.handleResponse(ctx, accumulator.ID, response); err != nil {
		return err
	}

	return nil
}

func (c *conversation) getNewCompletionParams() openai.ChatCompletionNewParams {
	return openai.ChatCompletionNewParams{
		MaxCompletionTokens: openai.Int(c.config.MaxTokens),
		Messages:            c.getSessionMessages(c.session),
		Model:               c.config.Model,
		N:                   openai.Int(1),
		Tools:               c.completionTools(c.session),
		Temperature:         openai.Float(c.config.Temperature),
	}
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
				assistantMsg.OfAssistant.ToolCalls = c.genericToolCallsToProvider(msg.ToolCalls...)
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

func (c *conversation) providerToolCallsToGeneric(toolCalls ...openai.ChatCompletionMessageToolCallUnionParam) (llms.ToolCalls, error) {
	results := make(llms.ToolCalls, len(toolCalls))

	for callNum, toolCall := range toolCalls {
		if toolCall.OfFunction == nil {
			tcType := toolCall.GetType()
			return nil, fmt.Errorf("got a tool call of type other than function: %s", *tcType)
		}

		name := toolCall.OfFunction.Function.Name

		args, err := c.session.Tools.GetArgs(name, []byte(toolCall.OfFunction.Function.Arguments))
		if err != nil {
			return nil, fmt.Errorf("failed to parse arguments for tool call to tool %q: %w", name, err)
		}

		results[callNum] = llms.ToolCall{
			ID:   toolCall.OfFunction.ID,
			Name: name,
			Args: args,
		}
	}

	return results, nil
}

func (c *conversation) genericToolCallsToProvider(toolCalls ...llms.ToolCall) []openai.ChatCompletionMessageToolCallUnionParam {
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

func (c *conversation) handleResponse(ctx context.Context, id string, response openai.ChatCompletionMessage) error {
	if response.ToParam().OfAssistant == nil {
		return fmt.Errorf("%w: no assistant message", llms.ErrEmptyResponse)
	}

	// We convert this ToParam() because it's the only way to get streaming responses to work...
	toolCalls, err := c.providerToolCallsToGeneric(response.ToParam().OfAssistant.ToolCalls...)
	if err != nil {
		return fmt.Errorf("failed to handle assistant tool calls: %w", err)
	}

	msg := c.newMessage(
		llms.WithID(id),
		llms.WithRole(llms.RoleAssistant),
		llms.WithContent(response.Content),
	)

	if len(toolCalls) > 0 {
		c.hasPendingToolCalls = true
		msg = msg.Update(llms.WithToolCalls(toolCalls...))

		c.emit(ctx, llms.EventToolCallsRequested{
			Message: msg,
		})
	} else {
		c.hasPendingToolCalls = false
		c.emit(ctx, llms.EventFinalMessage{
			Message: msg,
		})
	}

	return nil
}

func (c *conversation) waitForContinue(ctx context.Context) error {
	select {
	case <-c.continueChan:
		return nil
	case <-ctx.Done():
		return fmt.Errorf("context error while waiting for continue: %w", ctx.Err())
	}
}

// func (c *conversation) emit(ctx context.Context, e llms.Event) bool {
func (c *conversation) emit(ctx context.Context, e llms.Event) {
	select {
	case c.eventChan <- e:
		// return true
	case <-ctx.Done():
		// return false
	}
}
