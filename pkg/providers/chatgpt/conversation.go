package chatgpt

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/cneill/smoke/pkg/llms"
	"github.com/cneill/smoke/pkg/providers/base"
	"github.com/cneill/smoke/pkg/tools"
	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
	"github.com/openai/openai-go/v3/packages/param"
	"github.com/openai/openai-go/v3/responses"
	"github.com/openai/openai-go/v3/shared"
)

type conversation struct {
	*base.Conversation

	client openai.Client
}

func (c *conversation) sendNoStream(ctx context.Context) error {
	options := c.getNewResponsesParams()

	response, err := c.client.Responses.New(ctx, options, option.WithMaxRetries(5))
	if err != nil {
		return fmt.Errorf("%w: %w", llms.ErrCompletion, err)
	}

	return c.handleFinalResponse(ctx, response)
}

func (c *conversation) sendStream(ctx context.Context) error { //nolint:cyclop
	options := c.getNewResponsesParams()

	stream := c.client.Responses.NewStreaming(ctx, options, option.WithMaxRetries(5))
	defer stream.Close()

	var (
		finalResponse    *responses.Response
		reasoningSummary strings.Builder
	)

	for stream.Next() {
		switch evt := stream.Current().AsAny().(type) {
		case responses.ResponseTextDeltaEvent:
			c.Emit(ctx, llms.EventTextDelta{
				ID:   evt.ItemID,
				Text: evt.Delta,
			})
		case responses.ResponseReasoningSummaryTextDeltaEvent:
			reasoningSummary.WriteString(evt.Delta)

		case responses.ResponseCompletedEvent:
			finalResponse = &evt.Response

			slog.Debug("reasoning summary", "text", reasoningSummary.String())
		case responses.ResponseIncompleteEvent:
			slog.Warn("responses stream ended incomplete",
				"response_id", evt.Response.ID, "reason", evt.Response.IncompleteDetails.Reason)
			finalResponse = &evt.Response
		case responses.ResponseFailedEvent:
			if evt.Response.Error.Message != "" {
				return fmt.Errorf("%w: %s", llms.ErrCompletion, evt.Response.Error.Message)
			}

			return fmt.Errorf("%w: response failed", llms.ErrCompletion)
		default:
			slog.Debug("ignoring unhandled Responses stream event", "type", fmt.Sprintf("%T", evt))
		}
	}

	if err := stream.Err(); err != nil {
		return fmt.Errorf("%w: streaming: %w", llms.ErrCompletion, err)
	}

	if finalResponse == nil {
		return fmt.Errorf("%w: missing final response from stream", llms.ErrEmptyResponse)
	}

	return c.handleFinalResponse(ctx, finalResponse)
}

func (c *conversation) getNewResponsesParams() responses.ResponseNewParams {
	session := c.Session()
	config := c.Config()

	params := responses.ResponseNewParams{
		MaxOutputTokens: openai.Int(config.MaxTokens),
		Input:           c.getInputFromSession(session),
		Model:           config.ModelInfo.Model,
		Store:           openai.Bool(false),
		Temperature:     openai.Float(config.Temperature),
		Tools:           c.responsesTools(session.Tools.GetTools()),
	}

	if provider := c.Config().ModelInfo.Provider; provider == llms.LLMTypeChatGPT || provider == llms.LLMTypeGrok {
		params.Reasoning = shared.ReasoningParam{
			Effort:  shared.ReasoningEffort(c.Config().Effort), // "none", "minimal", "low", "medium", "high", "xhigh"
			Summary: shared.ReasoningSummaryDetailed,
		}
		params.Include = []responses.ResponseIncludable{responses.ResponseIncludableReasoningEncryptedContent}
	}

	return params
}

func (c *conversation) getInputFromSession(session *llms.Session) responses.ResponseNewParamsInputUnion { //nolint:cyclop,funlen
	inputItems := responses.ResponseInputParam{}

	for _, msg := range session.Messages {
		switch msg.Role {
		case llms.RoleAssistant:
			for _, block := range msg.Blocks {
				switch block.Type() { //nolint:exhaustive
				case llms.BlockTypeText:
					inputItems = append(inputItems, responses.ResponseInputItemUnionParam{
						OfMessage: &responses.EasyInputMessageParam{
							Content: responses.EasyInputMessageContentUnionParam{
								OfString: openai.String(block.Text.Text),
							},
							Role:  responses.EasyInputMessageRoleAssistant,
							Phase: responses.EasyInputMessagePhase(block.Text.Phase),
						},
					})
				case llms.BlockTypeToolCall:
					toolCall := block.ToolCall.ToolCall
					inputItems = append(inputItems, responses.ResponseInputItemUnionParam{
						OfFunctionCall: &responses.ResponseFunctionToolCallParam{
							CallID:    toolCall.ID,
							Name:      toolCall.Name,
							Arguments: toolCall.ArgsString(),
						},
					})
				case llms.BlockTypeReasoning:
					if block.Reasoning.Detail.Type() != llms.ReasoningDetailTypeOpenAI ||
						block.Reasoning.Detail.OpenAI.Source != c.Config().ModelInfo.Provider {
						continue
					}

					detail := block.Reasoning.Detail.OpenAI

					summary := make([]responses.ResponseReasoningItemSummaryParam, len(block.Reasoning.Summary))
					for i, text := range block.Reasoning.Summary {
						summary[i] = responses.ResponseReasoningItemSummaryParam{Text: text}
					}

					content := make([]responses.ResponseReasoningItemContentParam, len(detail.Content))
					for i, text := range detail.Content {
						content[i] = responses.ResponseReasoningItemContentParam{Text: text}
					}

					inputItems = append(inputItems, responses.ResponseInputItemUnionParam{
						OfReasoning: &responses.ResponseReasoningItemParam{
							ID:               detail.ID,
							Summary:          summary,
							Content:          content,
							EncryptedContent: openai.String(detail.EncryptedContent),
							Status:           responses.ResponseReasoningItemStatus(detail.Status),
						},
					})
				}
			}
		case llms.RoleSystem:
			inputItems = append(inputItems, responses.ResponseInputItemUnionParam{
				OfMessage: &responses.EasyInputMessageParam{
					Content: responses.EasyInputMessageContentUnionParam{
						OfString: openai.String(msg.TextContent()),
					},
					Role: responses.EasyInputMessageRoleSystem,
				},
			})
		case llms.RoleTool:
			toolCalls := msg.ToolCalls()
			if n := len(toolCalls); n != 1 {
				slog.Warn(
					"got wrong number of tool calls referenced in message with tool role (expecting 1); skipping",
					"num", n, "names", toolCalls.Names(),
				)

				continue
			}

			inputItems = append(inputItems, c.toolMessageInput(msg))

		case llms.RoleUser:
			inputItems = append(inputItems, responses.ResponseInputItemUnionParam{
				OfMessage: &responses.EasyInputMessageParam{
					Content: responses.EasyInputMessageContentUnionParam{
						OfString: openai.String(msg.TextContent()),
					},
					Role: responses.EasyInputMessageRoleUser,
				},
			})
		case llms.RoleUnknown:
			slog.Warn("got message with unknown role", "message", msg.TextContent())
		}
	}

	return responses.ResponseNewParamsInputUnion{
		OfInputItemList: inputItems,
	}
}

func (c *conversation) toolMessageInput(msg *llms.Message) responses.ResponseInputItemUnionParam {
	content := responses.ResponseInputItemFunctionCallOutputOutputUnionParam{}

	if len(msg.Images()) != 0 {
		content.OfResponseFunctionCallOutputItemArray = responses.ResponseFunctionCallOutputItemListParam{
			responses.ResponseFunctionCallOutputItemUnionParam{
				OfInputImage: &responses.ResponseInputImageContentParam{
					ImageURL: openai.String(msg.ImageB64URL()),
					Detail:   "auto",
				},
			},
		}
	} else {
		content.OfString = openai.String(msg.TextContent())
	}

	return responses.ResponseInputItemUnionParam{
		OfFunctionCallOutput: &responses.ResponseInputItemFunctionCallOutputParam{
			CallID: param.NewOpt[string](msg.ToolCalls()[0].ID),
			Output: content,
		},
	}
}

func (c *conversation) responsesTools(sessionTools tools.Tools) []responses.ToolUnionParam {
	responsesTools := make([]responses.ToolUnionParam, 0, len(sessionTools))

	for _, tool := range sessionTools {
		params := tool.Params()

		properties, err := params.JSONSchemaProperties()
		if err != nil {
			slog.Error("failed to get JSON Schema properties for tool, skipping", "tool_name", tool.Name(), "error", err)
			continue
		}

		toolUnion := responses.ToolUnionParam{
			OfFunction: &responses.FunctionToolParam{
				Name:        tool.Name(),
				Description: openai.String(tool.Description()),
				Strict:      openai.Bool(false),
				Parameters: openai.FunctionParameters{
					"type":       tools.ParamTypeObject,
					"properties": properties,
					"required":   params.RequiredKeys(),
				},
			},
		}
		responsesTools = append(responsesTools, toolUnion)
	}

	return responsesTools
}

func (c *conversation) handleFinalResponse(ctx context.Context, response *responses.Response) error {
	if response == nil || len(response.Output) == 0 {
		return fmt.Errorf("%w: no output returned", llms.ErrEmptyResponse)
	}

	c.Emit(ctx, llms.EventUsageUpdate{
		InputTokens:  response.Usage.InputTokens,
		OutputTokens: response.Usage.OutputTokens,
	})

	msg, err := c.outputToMessage(response.Output)
	if err != nil {
		return err
	}

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

func (c *conversation) newToolCall(id, name, rawArgs string) llms.ToolCall {
	args, err := c.Session().Tools.GetArgs(name, []byte(rawArgs))

	toolCall := llms.ToolCall{
		ID:      id,
		Name:    name,
		Args:    args,
		RawArgs: rawArgs,
	}
	if err != nil {
		toolCall.ArgsError = fmt.Sprintf("failed to parse arguments for tool call to tool %q: %v", name, err)
	}

	return toolCall
}

func (c *conversation) outputToMessage(output []responses.ResponseOutputItemUnion) (*llms.Message, error) { //nolint:cyclop
	msgOpts := []llms.MessageOpt{llms.WithRole(llms.RoleAssistant)}
	blocks := make([]llms.Block, 0, len(output))
	messageIDSet := false

	for _, item := range output {
		switch outputItem := item.AsAny().(type) {
		case responses.ResponseOutputMessage:
			if !messageIDSet {
				msgOpts = append(msgOpts, llms.WithID(outputItem.ID))
				messageIDSet = true
			}

			for _, contentItem := range outputItem.Content {
				switch content := contentItem.AsAny().(type) {
				case responses.ResponseOutputText:
					blocks = append(blocks, llms.PhasedTextContentBlock(content.Text, string(outputItem.Phase)))
				case responses.ResponseOutputRefusal:
					return nil, fmt.Errorf("%w: %s", llms.ErrPromptRefused, content.Refusal)
				}
			}
		case responses.ResponseFunctionToolCall:
			blocks = append(blocks, llms.ToolContentBlock(c.newToolCall(outputItem.CallID, outputItem.Name, outputItem.Arguments)))
		case responses.ResponseReasoningItem:
			summary := make([]string, len(outputItem.Summary))
			for i, part := range outputItem.Summary {
				summary[i] = part.Text
			}

			content := make([]string, len(outputItem.Content))
			for i, part := range outputItem.Content {
				content[i] = part.Text
			}

			blocks = append(blocks, llms.ReasoningContentBlock(llms.ReasoningBlock{
				Summary: summary,
				Detail: llms.ReasoningDetail{
					OpenAI: &llms.OpenAIReasoningDetail{
						Source:           c.Config().ModelInfo.Provider,
						ID:               outputItem.ID,
						EncryptedContent: outputItem.EncryptedContent,
						Content:          content,
						Status:           string(outputItem.Status),
					},
				},
			}))
		}
	}

	msgOpts = append(msgOpts, llms.WithBlocks(blocks...))

	return c.NewMessage(msgOpts...), nil
}
