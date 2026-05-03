package chatgpt

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/cneill/smoke/pkg/llms"
	"github.com/cneill/smoke/pkg/providers/base"
	"github.com/cneill/smoke/pkg/tools"
	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
)

type conversation struct {
	*base.Conversation

	client openai.Client
}

func (c *conversation) sendNoStream(ctx context.Context) error {
	options := c.getNewCompletionParams()

	result, err := c.client.Chat.Completions.New(ctx, options, option.WithMaxRetries(5))
	if err != nil {
		return fmt.Errorf("%w: %w", llms.ErrCompletion, err)
	}

	if len(result.Choices) == 0 {
		return fmt.Errorf("%w: no messages returned", llms.ErrEmptyResponse)
	}

	if refusal := result.Choices[0].Message.Refusal; refusal != "" {
		return fmt.Errorf("%w: %s", llms.ErrPromptRefused, refusal)
	}

	c.Emit(ctx, llms.EventUsageUpdate{
		InputTokens:  result.Usage.PromptTokens,
		OutputTokens: result.Usage.CompletionTokens,
	})

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
		if !accumulator.AddChunk(chunk) {
			slog.Warn("failed to accumulate new conversation chunk")
		}

		// TODO: need either of these "JustFinishedX" checks?
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

		c.Emit(ctx, llms.EventTextDelta{
			ID:   accumulator.ID,
			Text: chunk.Choices[0].Delta.Content,
		})
	}

	if err := stream.Err(); err != nil {
		return fmt.Errorf("%w: streaming: %w", llms.ErrCompletion, err)
	}

	if len(accumulator.Choices) == 0 {
		return fmt.Errorf("%w: no messages returned", llms.ErrEmptyResponse)
	}

	c.Emit(ctx, llms.EventUsageUpdate{
		InputTokens:  accumulator.Usage.PromptTokens,
		OutputTokens: accumulator.Usage.CompletionTokens,
	})

	response := accumulator.Choices[0].Message

	if err := c.handleResponse(ctx, accumulator.ID, response); err != nil {
		return err
	}

	return nil
}

func (c *conversation) getNewCompletionParams() openai.ChatCompletionNewParams {
	session := c.Session()
	config := c.Config()

	return openai.ChatCompletionNewParams{
		MaxCompletionTokens: openai.Int(config.MaxTokens),
		Messages:            c.getSessionMessages(session),
		Model:               config.Model,
		N:                   openai.Int(1),
		Tools:               c.completionTools(session),
		Temperature:         openai.Float(config.Temperature),
	}
}

// getSessionMessages converts the generic messages in 'session' to messages appropriate for a ChatGPT conversation
// history.
func (c *conversation) getSessionMessages(session *llms.Session) []openai.ChatCompletionMessageParamUnion {
	results := make([]openai.ChatCompletionMessageParamUnion, len(session.Messages))

	for num, msg := range session.Messages {
		switch msg.Role {
		case llms.RoleAssistant:
			assistantMsg := openai.AssistantMessage(msg.TextContent)

			if msg.HasToolCalls() {
				assistantMsg.OfAssistant.ToolCalls = c.genericToolCallsToProvider(msg.ToolCalls...)
			}

			results[num] = assistantMsg
		case llms.RoleSystem:
			results[num] = openai.SystemMessage(msg.TextContent)
		case llms.RoleUser:
			results[num] = openai.UserMessage(msg.TextContent)
		case llms.RoleTool:
			if n := len(msg.ToolCalls); n > 1 {
				slog.Warn("more than one tool call referenced in message with tool role; skipping", "num", n, "names", msg.ToolCalls.Names())
				continue
			}

			results[num] = openai.ToolMessage(msg.TextContent, msg.ToolCalls[0].ID)
		case llms.RoleUnknown:
			slog.Warn("got message with unknown role", "message", msg.TextContent)
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

		args, err := c.Session().Tools.GetArgs(name, []byte(toolCall.OfFunction.Function.Arguments))
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

	msg := c.NewMessage(
		llms.WithID(id),
		llms.WithRole(llms.RoleAssistant),
		llms.WithTextContent(response.Content),
	)

	if len(toolCalls) > 0 {
		c.HasPendingToolCalls = true
		msg = msg.Update(llms.WithToolCalls(toolCalls...))

		c.Emit(ctx, llms.EventToolCallsRequested{
			Message: msg,
		})
	} else {
		c.HasPendingToolCalls = false
		c.Emit(ctx, llms.EventFinalMessage{
			Message: msg,
		})
	}

	return nil
}
