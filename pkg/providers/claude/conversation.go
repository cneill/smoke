package claude

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
	"github.com/cneill/smoke/pkg/llms"
	"github.com/cneill/smoke/pkg/providers/base"
)

type conversation struct {
	*base.Conversation

	client anthropic.Client
}

func (c *conversation) sendNoStream(ctx context.Context) error {
	messageParams := c.getMessageNewParams()

	result, err := c.client.Messages.New(ctx, messageParams)
	if err != nil {
		return fmt.Errorf("%w: %w", llms.ErrCompletion, err)
	}

	c.Emit(ctx, llms.EventUsageUpdate{
		InputTokens:  result.Usage.InputTokens,
		OutputTokens: result.Usage.OutputTokens,
	})

	if len(result.Content) == 0 {
		return fmt.Errorf("%w: no messages returned", llms.ErrEmptyResponse)
	}

	if result.StopReason == anthropic.StopReasonRefusal {
		return fmt.Errorf("%w: %s", llms.ErrPromptRefused, result.Content[0].Text)
	}

	if err := c.handleResponse(ctx, result.ID, result.Content); err != nil {
		return err
	}

	return nil
}

func (c *conversation) sendStream(ctx context.Context) error {
	messageParams := c.getMessageNewParams()

	stream := c.client.Messages.NewStreaming(ctx, messageParams, option.WithMaxRetries(5))
	defer stream.Close()

	accumulator := anthropic.Message{}

	for stream.Next() {
		chunk := stream.Current()
		if err := accumulator.Accumulate(chunk); err != nil {
			return fmt.Errorf("failed to handle message chunk: %w", err)
		}

		chunkType, ok := chunk.AsAny().(anthropic.ContentBlockDeltaEvent)
		if !ok {
			slog.Warn("unknown chunk type", "type", fmt.Sprintf("%T", chunk.AsAny()))
			continue
		}

		// TODO: handle MessageStartEvent, ContentBlockStartEvent, ContentBlockStopEvent, MessageDeltaEvent,
		// MessageStopEvent, etc

		switch deltaType := chunkType.Delta.AsAny().(type) {
		case anthropic.TextDelta:
			c.Emit(ctx, llms.EventTextDelta{
				ID:   accumulator.ID,
				Text: deltaType.Text,
			})
		// TODO: handle InputJSONDelta type?
		default:
			slog.Warn("unknown delta type", "type", fmt.Sprintf("%T", chunkType.Delta.AsAny()))
			continue
		}
	}

	if err := stream.Err(); err != nil {
		return fmt.Errorf("%w: streaming: %w", llms.ErrCompletion, err)
	}

	if len(accumulator.Content) == 0 {
		return fmt.Errorf("%w: no messages returned", llms.ErrEmptyResponse)
	}

	if accumulator.StopReason == anthropic.StopReasonRefusal {
		return fmt.Errorf("%w: %s", llms.ErrPromptRefused, accumulator.Content[0].Text)
	}

	c.Emit(ctx, llms.EventUsageUpdate{
		InputTokens:  accumulator.Usage.InputTokens,
		OutputTokens: accumulator.Usage.OutputTokens,
	})

	if err := c.handleResponse(ctx, accumulator.ID, accumulator.Content); err != nil {
		return err
	}

	return nil
}

func (c *conversation) getMessageNewParams() anthropic.MessageNewParams {
	session := c.Session()
	config := c.Config()

	return anthropic.MessageNewParams{
		Messages:  c.getSessionMessages(session),
		MaxTokens: config.MaxTokens,
		Model:     config.Model,
		System: []anthropic.TextBlockParam{
			{Text: session.SystemMessage},
		},
		Tools:        c.newMessageTools(session),
		Temperature:  anthropic.Float(config.Temperature),
		CacheControl: anthropic.NewCacheControlEphemeralParam(),
	}
}

func (c *conversation) getSessionMessages(session *llms.Session) []anthropic.MessageParam {
	results := make([]anthropic.MessageParam, len(session.Messages))

	for num, msg := range session.Messages {
		switch msg.Role {
		case llms.RoleAssistant:
			contentBlocks := []anthropic.ContentBlockParamUnion{}

			if strings.TrimSpace(msg.TextContent) != "" {
				contentBlocks = append(contentBlocks, anthropic.NewTextBlock(msg.TextContent))
			}

			if msg.HasToolCalls() {
				contentBlocks = append(contentBlocks, c.genericToolCallsToProvider(msg.ToolCalls...)...)
			}

			results[num] = anthropic.NewAssistantMessage(contentBlocks...)
		case llms.RoleSystem:
			// Anthropic defines the system prompt outside of messages
		case llms.RoleUser:
			results[num] = anthropic.NewUserMessage(anthropic.NewTextBlock(msg.TextContent))
		case llms.RoleTool:
			if n := len(msg.ToolCalls); n > 1 {
				slog.Warn("more than one tool call referenced in message with tool role; skipping", "num", n, "names", msg.ToolCalls.Names())
				continue
			}

			content := msg.TextContent
			if content == "" {
				content = "[no output]" // can't be empty?
			}

			results[num] = anthropic.NewUserMessage(anthropic.NewToolResultBlock(msg.ToolCalls[0].ID, content, msg.Error != ""))

		case llms.RoleUnknown:
			slog.Warn("got message with unknown role", "message", msg.TextContent)
		}
	}

	return results
}

func (c *conversation) newMessageTools(session *llms.Session) []anthropic.ToolUnionParam {
	results := []anthropic.ToolUnionParam{}

	for _, tool := range session.Tools.GetTools() {
		params := tool.Params()

		properties, err := params.JSONSchemaProperties()
		if err != nil {
			slog.Error("failed to get JSON Schema properties for tool, skipping", "tool_name", tool.Name(), "error", err)
			continue
		}

		toolDef := anthropic.ToolParam{
			Name:        tool.Name(),
			Description: anthropic.String(tool.Description()),
			InputSchema: anthropic.ToolInputSchemaParam{
				Type:       "object",
				Properties: properties,
				Required:   params.RequiredKeys(),
			},
		}

		results = append(results, anthropic.ToolUnionParam{OfTool: &toolDef})
	}

	return results
}

func (c *conversation) providerToolCallsToGeneric(toolCalls ...anthropic.ToolUseBlock) (llms.ToolCalls, error) {
	results := make(llms.ToolCalls, len(toolCalls))

	for callNum, toolCall := range toolCalls {
		args, err := c.Session().Tools.GetArgs(toolCall.Name, toolCall.Input)
		if err != nil {
			return nil, fmt.Errorf("failed to parse arguments for tool call to tool %q: %w", toolCall.Name, err)
		}

		results[callNum] = llms.ToolCall{
			ID:   toolCall.ID,
			Name: toolCall.Name,
			Args: args,
		}
	}

	return results, nil
}

func (c *conversation) genericToolCallsToProvider(toolCalls ...llms.ToolCall) []anthropic.ContentBlockParamUnion {
	results := make([]anthropic.ContentBlockParamUnion, len(toolCalls))

	for callNum, toolCall := range toolCalls {
		results[callNum] = anthropic.ContentBlockParamUnion{
			OfToolUse: &anthropic.ToolUseBlockParam{
				ID:    toolCall.ID,
				Name:  toolCall.Name,
				Input: toolCall.Args,
			},
		}
	}

	return results
}

func (c *conversation) handleResponse(ctx context.Context, id string, blocks []anthropic.ContentBlockUnion) error {
	textBuilder := strings.Builder{}
	providerToolCalls := []anthropic.ToolUseBlock{}

	for _, block := range blocks {
		switch block := block.AsAny().(type) {
		case anthropic.TextBlock:
			// TODO: citations?
			if strings.TrimSpace(block.Text) != "" {
				textBuilder.WriteString(block.Text + "\n")
			}
		case anthropic.ToolUseBlock:
			providerToolCalls = append(providerToolCalls, block)
		}
	}

	msg := c.NewMessage(
		llms.WithID(id),
		llms.WithRole(llms.RoleAssistant),
		llms.WithTextContent(textBuilder.String()),
	)

	if len(providerToolCalls) > 0 {
		c.HasPendingToolCalls = true

		toolCalls, err := c.providerToolCallsToGeneric(providerToolCalls...)
		if err != nil {
			return fmt.Errorf("failed to handle assistant tool calls: %w", err)
		}

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
