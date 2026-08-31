package claude

import (
	"context"
	"encoding/base64"
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
		// Anthropic reports input tokens split across three buckets depending on prompt caching: total input
		// tokens is the sum of InputTokens, CacheCreationInputTokens, and CacheReadInputTokens. Since we always
		// set CacheControl on requests, most turns report the bulk of input under the cache fields, so we must
		// sum all three or usage totals will fluctuate wildly instead of growing monotonically.
		InputTokens:  result.Usage.InputTokens + result.Usage.CacheCreationInputTokens + result.Usage.CacheReadInputTokens,
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

func (c *conversation) sendStream(ctx context.Context) error { //nolint:cyclop
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
			// slog.Warn("unknown chunk type", "type", fmt.Sprintf("%T", chunk.AsAny()))
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
		case anthropic.ThinkingDelta:
			// slog.Debug("Thinking...", "text", deltaType.Thinking)
		case anthropic.InputJSONDelta:
			// ignore - partial JSON is useless except to say "working on a tool call", perhaps
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
		// See sendNoStream for why we sum all three input token buckets.
		InputTokens: accumulator.Usage.InputTokens + accumulator.Usage.CacheCreationInputTokens +
			accumulator.Usage.CacheReadInputTokens,
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
		OutputConfig: anthropic.OutputConfigParam{
			Effort: anthropic.OutputConfigEffort(c.Config().Effort), // "low", "medium", "high", "xhigh", "max"
		},
		Thinking: anthropic.ThinkingConfigParamUnion{
			OfAdaptive: &anthropic.ThinkingConfigAdaptiveParam{
				Display: anthropic.ThinkingConfigAdaptiveDisplaySummarized,
			},
		},
	}
}

func (c *conversation) getSessionMessages(session *llms.Session) []anthropic.MessageParam { //nolint:cyclop
	results := make([]anthropic.MessageParam, len(session.Messages))

	for idx, msg := range session.Messages {
		switch msg.Role { //nolint:exhaustive
		case llms.RoleAssistant:
			contentBlocks := []anthropic.ContentBlockParamUnion{}

			for _, block := range msg.Blocks {
				switch block.Type() { //nolint:exhaustive
				case llms.BlockTypeText:
					if strings.TrimSpace(block.Text.Text) != "" {
						contentBlocks = append(contentBlocks, anthropic.NewTextBlock(block.Text.Text))
					}
				case llms.BlockTypeToolCall:
					contentBlocks = append(contentBlocks, c.genericToolCallsToProvider(block.ToolCall.ToolCall)...)
				case llms.BlockTypeReasoning:
					if block.Reasoning.Detail.Type() != llms.ReasoningDetailTypeAnthropic ||
						block.Reasoning.Detail.Anthropic.Source != llms.LLMTypeClaude {
						continue
					}

					detail := block.Reasoning.Detail.Anthropic
					if detail.Redacted {
						contentBlocks = append(contentBlocks, anthropic.NewRedactedThinkingBlock(detail.Data))
					} else {
						contentBlocks = append(contentBlocks, anthropic.NewThinkingBlock(detail.Signature, detail.Thinking))
					}
				}
			}

			results[idx] = anthropic.NewAssistantMessage(contentBlocks...)
		case llms.RoleSystem:
			// Anthropic defines the system prompt outside of messages
		case llms.RoleUser:
			results[idx] = anthropic.NewUserMessage(anthropic.NewTextBlock(msg.TextContent()))
		case llms.RoleTool:
			results[idx] = c.genericToolResponseMessageToProvider(msg)
		default:
			slog.Warn("got message with unknown role", "message", msg.TextContent(), "role", msg.Role)
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

func (c *conversation) providerToolCallsToGeneric(toolCalls ...anthropic.ToolUseBlock) llms.ToolCalls {
	results := make(llms.ToolCalls, len(toolCalls))

	for callNum, toolCall := range toolCalls {
		args, err := c.Session().Tools.GetArgs(toolCall.Name, toolCall.Input)

		results[callNum] = llms.ToolCall{
			ID:      toolCall.ID,
			Name:    toolCall.Name,
			Args:    args,
			RawArgs: string(toolCall.Input),
		}
		if err != nil {
			results[callNum].ArgsError = fmt.Sprintf("failed to parse arguments for tool call to tool %q: %v",
				toolCall.Name, err)
		}
	}

	return results
}

func (c *conversation) genericToolCallsToProvider(toolCalls ...llms.ToolCall) []anthropic.ContentBlockParamUnion {
	results := make([]anthropic.ContentBlockParamUnion, len(toolCalls))

	for callNum, toolCall := range toolCalls {
		results[callNum] = anthropic.ContentBlockParamUnion{
			OfToolUse: &anthropic.ToolUseBlockParam{
				ID:    toolCall.ID,
				Name:  toolCall.Name,
				Input: toolCall.ProviderArgs(),
			},
		}
	}

	return results
}

func (c *conversation) genericToolResponseMessageToProvider(msg *llms.Message) anthropic.MessageParam {
	var toolContent anthropic.ToolResultBlockParamContentUnion

	images := msg.Images()
	toolCalls := msg.ToolCalls()

	if len(images) != 0 {
		toolContent = anthropic.ToolResultBlockParamContentUnion{
			OfImage: &anthropic.ImageBlockParam{
				Source: anthropic.ImageBlockParamSourceUnion{
					OfBase64: &anthropic.Base64ImageSourceParam{
						Data:      base64.StdEncoding.EncodeToString(images[0].Data),
						MediaType: anthropic.Base64ImageSourceMediaType(images[0].MediaType),
					},
				},
			},
		}
	} else {
		textContent := msg.TextContent()
		if textContent == "" {
			textContent = "[no output]"
		}

		toolContent = anthropic.ToolResultBlockParamContentUnion{
			OfText: &anthropic.TextBlockParam{
				Text: textContent,
			},
		}
	}

	return anthropic.MessageParam{
		Role: anthropic.MessageParamRoleUser,
		Content: []anthropic.ContentBlockParamUnion{
			{
				OfToolResult: &anthropic.ToolResultBlockParam{
					ToolUseID: toolCalls[0].ID,
					IsError:   anthropic.Bool(msg.Error != ""),
					Content: []anthropic.ToolResultBlockParamContentUnion{
						toolContent,
					},
				},
			},
		},
	}
}

func (c *conversation) handleResponse(ctx context.Context, id string, providerBlocks []anthropic.ContentBlockUnion) error {
	blocks := make([]llms.Block, 0, len(providerBlocks))
	for _, providerBlock := range providerBlocks {
		switch block := providerBlock.AsAny().(type) {
		case anthropic.TextBlock:
			if strings.TrimSpace(block.Text) != "" {
				blocks = append(blocks, llms.TextContentBlock(block.Text))
			}
		case anthropic.ToolUseBlock:
			calls := c.providerToolCallsToGeneric(block)
			blocks = append(blocks, llms.ToolContentBlock(calls[0]))
		case anthropic.ThinkingBlock:
			blocks = append(blocks, llms.ReasoningContentBlock(llms.ReasoningBlock{
				Summary: []string{block.Thinking},
				Detail: llms.ReasoningDetail{
					Anthropic: &llms.AnthropicReasoningDetail{
						Source:    llms.LLMTypeClaude,
						Thinking:  block.Thinking,
						Signature: block.Signature,
					},
				},
			}))
		case anthropic.RedactedThinkingBlock:
			blocks = append(blocks, llms.ReasoningContentBlock(llms.ReasoningBlock{
				Detail: llms.ReasoningDetail{
					Anthropic: &llms.AnthropicReasoningDetail{
						Source:   llms.LLMTypeClaude,
						Data:     block.Data,
						Redacted: true,
					},
				},
			}))
		}
	}

	msg := c.NewMessage(
		llms.WithID(id),
		llms.WithRole(llms.RoleAssistant),
		llms.WithBlocks(blocks...),
	)

	if msg.HasToolCalls() {
		c.HasPendingToolCalls = true

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
