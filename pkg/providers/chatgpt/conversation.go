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

func (c *conversation) sendStream(ctx context.Context) error {
	options := c.getNewResponsesParams()

	stream := c.client.Responses.NewStreaming(ctx, options, option.WithMaxRetries(5))
	defer stream.Close()

	var finalResponse *responses.Response

	for stream.Next() {
		switch evt := stream.Current().AsAny().(type) {
		case responses.ResponseTextDeltaEvent:
			c.Emit(ctx, llms.EventTextDelta{
				ID:   evt.ItemID,
				Text: evt.Delta,
			})
		case responses.ResponseCompletedEvent:
			finalResponse = &evt.Response
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
		Model:           config.Model,
		Store:           openai.Bool(false),
		Temperature:     openai.Float(config.Temperature),
		Tools:           c.responsesTools(session.Tools.GetTools()),
	}

	if c.Config().Provider == llms.LLMTypeChatGPT || c.Config().Provider == llms.LLMTypeGrok {
		params.Reasoning = shared.ReasoningParam{
			Effort: shared.ReasoningEffort(c.Config().Effort), // "none", "minimal", "low", "medium", "high", "xhigh"
		}
	}

	return params
}

func (c *conversation) getInputFromSession(session *llms.Session) responses.ResponseNewParamsInputUnion {
	inputItems := responses.ResponseInputParam{}

	for _, msg := range session.Messages {
		switch msg.Role {
		case llms.RoleAssistant:
			inputItems = append(inputItems, responses.ResponseInputItemUnionParam{
				OfMessage: &responses.EasyInputMessageParam{
					Content: responses.EasyInputMessageContentUnionParam{
						OfString: openai.String(msg.TextContent),
					},
					Role: responses.EasyInputMessageRoleAssistant,
					// TODO: handle Phase in llms.Message?!
				},
			})

			if msg.HasToolCalls() {
				for _, toolCall := range msg.ToolCalls {
					inputItems = append(inputItems, responses.ResponseInputItemUnionParam{
						OfFunctionCall: &responses.ResponseFunctionToolCallParam{
							CallID:    toolCall.ID,
							Name:      toolCall.Name,
							Arguments: toolCall.ArgsString(),
						},
					})
				}
			}
		case llms.RoleSystem:
			inputItems = append(inputItems, responses.ResponseInputItemUnionParam{
				OfMessage: &responses.EasyInputMessageParam{
					Content: responses.EasyInputMessageContentUnionParam{
						OfString: openai.String(msg.TextContent),
					},
					Role: responses.EasyInputMessageRoleSystem,
				},
			})
		case llms.RoleTool:
			if n := len(msg.ToolCalls); n != 1 {
				slog.Warn(
					"got wrong number of tool calls referenced in message with tool role (expecting 1); skipping",
					"num", n, "names", msg.ToolCalls.Names(),
				)

				continue
			}

			var content responses.ResponseInputItemFunctionCallOutputOutputUnionParam

			if len(msg.ImageContent) != 0 {
				content = responses.ResponseInputItemFunctionCallOutputOutputUnionParam{
					OfResponseFunctionCallOutputItemArray: responses.ResponseFunctionCallOutputItemListParam{
						responses.ResponseFunctionCallOutputItemUnionParam{
							OfInputImage: &responses.ResponseInputImageContentParam{
								ImageURL: openai.String(msg.ImageB64URL()),
								Detail:   "auto",
							},
						},
					},
				}
			} else {
				content = responses.ResponseInputItemFunctionCallOutputOutputUnionParam{
					OfString: openai.String(msg.TextContent),
				}
			}

			inputItems = append(inputItems, responses.ResponseInputItemUnionParam{
				OfFunctionCallOutput: &responses.ResponseInputItemFunctionCallOutputParam{
					CallID: msg.ToolCalls[0].ID,
					Output: content,
				},
			})

		case llms.RoleUser:
			inputItems = append(inputItems, responses.ResponseInputItemUnionParam{
				OfMessage: &responses.EasyInputMessageParam{
					Content: responses.EasyInputMessageContentUnionParam{
						OfString: openai.String(msg.TextContent),
					},
					Role: responses.EasyInputMessageRoleUser,
				},
			})
		case llms.RoleUnknown:
			slog.Warn("got message with unknown role", "message", msg.TextContent)
		}
	}

	return responses.ResponseNewParamsInputUnion{
		OfInputItemList: inputItems,
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

func (c *conversation) outputToMessage(output []responses.ResponseOutputItemUnion) (*llms.Message, error) {
	var (
		msgOpts = []llms.MessageOpt{
			llms.WithRole(llms.RoleAssistant),
		}

		messageBuilder strings.Builder
		toolCalls      llms.ToolCalls
	)

	for _, item := range output {
		switch outputItem := item.AsAny().(type) {
		case responses.ResponseOutputMessage:
			msgOpts = append(msgOpts, llms.WithID(outputItem.ID))

			for _, contentItem := range outputItem.Content {
				switch content := contentItem.AsAny().(type) {
				case responses.ResponseOutputText:
					messageBuilder.WriteString(content.Text)
				case responses.ResponseOutputRefusal:
					return nil, fmt.Errorf("%w: %s", llms.ErrPromptRefused, content.Refusal)
				}
			}
		case responses.ResponseFunctionToolCall:
			toolCalls = append(toolCalls, c.newToolCall(outputItem.CallID, outputItem.Name, outputItem.Arguments))
		case responses.ResponseReasoningItem:
			// slog.Debug("Got reasoning", "reasoning", outputItem.Summary)
		}
	}

	msgOpts = append(msgOpts, llms.WithTextContent(messageBuilder.String()))
	msgOpts = append(msgOpts, llms.WithToolCalls(toolCalls...))

	return c.NewMessage(msgOpts...), nil
}
