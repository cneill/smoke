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

type responsesStreamState struct {
	assistantMsgID string
	finalResponse  *responses.Response
}

func (c *conversation) sendNoStream(ctx context.Context) error {
	options := c.getNewResponsesParams()

	response, err := c.client.Responses.New(ctx, options, option.WithMaxRetries(5))
	if err != nil {
		return fmt.Errorf("%w: %w", llms.ErrCompletion, err)
	}

	c.Emit(ctx, llms.EventUsageUpdate{
		InputTokens:  response.Usage.InputTokens,
		OutputTokens: response.Usage.OutputTokens,
	})

	if len(response.Output) == 0 {
		return fmt.Errorf("%w: no output returned", llms.ErrEmptyResponse)
	}

	if err := c.handleResponseOutput(ctx, response.Output); err != nil {
		return err
	}

	return nil
}

func (c *conversation) sendStream(ctx context.Context) error {
	options := c.getNewResponsesParams()

	stream := c.client.Responses.NewStreaming(ctx, options, option.WithMaxRetries(5))
	defer stream.Close()

	state := &responsesStreamState{}

	for stream.Next() {
		if err := c.handleResponsesStreamEvent(ctx, state, stream.Current()); err != nil {
			return err
		}
	}

	if err := stream.Err(); err != nil {
		return fmt.Errorf("%w: streaming: %w", llms.ErrCompletion, err)
	}

	return c.finishResponsesStream(ctx, state)
}

func (c *conversation) handleResponsesStreamEvent(
	ctx context.Context,
	state *responsesStreamState,
	event responses.ResponseStreamEventUnion,
) error {
	switch evt := event.AsAny().(type) {
	case responses.ResponseCreatedEvent:
		state.assistantMsgID = evt.Response.ID
	case responses.ResponseTextDeltaEvent:
		c.Emit(ctx, llms.EventTextDelta{
			ID:   state.assistantMsgID,
			Text: evt.Delta,
		})
	case responses.ResponseRefusalDeltaEvent:
		// Refusals are handled from the finalized output.
	case responses.ResponseOutputItemAddedEvent:
		c.captureResponsesStreamOutputItem(state, evt.Item)
	case responses.ResponseOutputItemDoneEvent:
		c.captureResponsesStreamOutputItem(state, evt.Item)
	case responses.ResponseCompletedEvent:
		state.finalResponse = &evt.Response
	case responses.ResponseFailedEvent:
		return c.responseFailedError(evt)
	case responses.ResponseIncompleteEvent:
		slog.Warn("responses stream ended incomplete", "response_id", evt.Response.ID, "reason", evt.Response.IncompleteDetails.Reason)
		state.finalResponse = &evt.Response
	default:
		c.handleIgnoredResponsesStreamEvent(event)
	}

	return nil
}

func (c *conversation) responseFailedError(evt responses.ResponseFailedEvent) error {
	if evt.Response.Error.Message != "" {
		return fmt.Errorf("%w: %s", llms.ErrCompletion, evt.Response.Error.Message)
	}

	return fmt.Errorf("%w: response failed", llms.ErrCompletion)
}

func (c *conversation) handleIgnoredResponsesStreamEvent(event responses.ResponseStreamEventUnion) {
	switch event.AsAny().(type) {
	case responses.ResponseInProgressEvent,
		responses.ResponseQueuedEvent,
		responses.ResponseFunctionCallArgumentsDeltaEvent,
		responses.ResponseFunctionCallArgumentsDoneEvent,
		responses.ResponseOutputTextAnnotationAddedEvent,
		responses.ResponseTextDoneEvent,
		responses.ResponseRefusalDoneEvent,
		responses.ResponseContentPartAddedEvent,
		responses.ResponseContentPartDoneEvent,
		responses.ResponseReasoningSummaryPartAddedEvent,
		responses.ResponseReasoningSummaryPartDoneEvent,
		responses.ResponseReasoningSummaryTextDeltaEvent,
		responses.ResponseReasoningSummaryTextDoneEvent,
		responses.ResponseReasoningTextDeltaEvent,
		responses.ResponseReasoningTextDoneEvent:
		return
	default:
		slog.Debug("ignoring unhandled Responses stream event", "type", fmt.Sprintf("%T", event.AsAny()))
	}
}

func (c *conversation) captureResponsesStreamOutputItem(
	state *responsesStreamState,
	item responses.ResponseOutputItemUnion,
) {
	msg, ok := item.AsAny().(responses.ResponseOutputMessage)
	if !ok {
		return
	}

	if msg.ID != "" {
		state.assistantMsgID = msg.ID
	}
}

func (c *conversation) finishResponsesStream(ctx context.Context, state *responsesStreamState) error {
	if state.finalResponse == nil {
		return fmt.Errorf("%w: missing final response from stream", llms.ErrEmptyResponse)
	}

	if len(state.finalResponse.Output) == 0 {
		return fmt.Errorf("%w: no output returned", llms.ErrEmptyResponse)
	}

	c.Emit(ctx, llms.EventUsageUpdate{
		InputTokens:  state.finalResponse.Usage.InputTokens,
		OutputTokens: state.finalResponse.Usage.OutputTokens,
	})

	return c.handleResponseOutput(ctx, state.finalResponse.Output)
}

func (c *conversation) getNewResponsesParams() responses.ResponseNewParams {
	session := c.Session()
	config := c.Config()

	params := responses.ResponseNewParams{
		MaxOutputTokens: openai.Int(config.MaxTokens),
		Input:           c.getSessionInput(session),
		Model:           config.Model,
		Store:           openai.Bool(false),
		Temperature:     openai.Float(config.Temperature),
		Tools:           c.responsesTools(session.Tools.GetTools()),
	}

	// Grok doesn't support this
	if c.Config().Provider == llms.LLMTypeChatGPT {
		params.Reasoning = shared.ReasoningParam{
			Effort:  shared.ReasoningEffortMedium,
			Summary: shared.ReasoningSummaryConcise,
		}
	}

	return params
}

func (c *conversation) getSessionInput(session *llms.Session) responses.ResponseNewParamsInputUnion {
	inputItems := responses.ResponseInputParam{}
	// inputItems := make(responses.ResponseInputParam, len(session.Messages))

	for _, msg := range session.Messages {
		switch msg.Role {
		case llms.RoleAssistant:
			inputItems = append(inputItems, responses.ResponseInputItemUnionParam{
				OfMessage: &responses.EasyInputMessageParam{
					Content: responses.EasyInputMessageContentUnionParam{
						OfString: openai.String(msg.TextContent),
					},
					Role: responses.EasyInputMessageRoleAssistant,
					// TODO: handle Phase?!
				},
			})

			if msg.HasToolCalls() {
				for _, toolCall := range msg.ToolCalls {
					inputItems = append(inputItems, responses.ResponseInputItemUnionParam{
						OfFunctionCall: &responses.ResponseFunctionToolCallParam{
							CallID:    toolCall.ID,
							Name:      toolCall.Name,
							Arguments: toolCall.Args.String(),
						},
					})
				}
			}
			// TODO: handle tool calls
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

			inputItems = append(inputItems, responses.ResponseInputItemUnionParam{
				OfFunctionCallOutput: &responses.ResponseInputItemFunctionCallOutputParam{
					CallID: msg.ToolCalls[0].ID,
					Output: responses.ResponseInputItemFunctionCallOutputOutputUnionParam{
						OfString: openai.String(msg.TextContent),
					},
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

func (c *conversation) handleResponseOutput(ctx context.Context, output []responses.ResponseOutputItemUnion) error {
	msg, toolCalls, err := c.responseMessageAndToolCalls(output)
	if err != nil {
		return err
	}

	if len(toolCalls) > 0 {
		c.HasPendingToolCalls = true
		msg = msg.Update(llms.WithToolCalls(toolCalls...))

		c.Emit(ctx, llms.EventToolCallsRequested{
			Message: msg,
		})

		return nil
	}

	c.HasPendingToolCalls = false
	c.Emit(ctx, llms.EventFinalMessage{
		Message: msg,
	})

	return nil
}

func (c *conversation) responseMessageAndToolCalls(output []responses.ResponseOutputItemUnion) (*llms.Message, llms.ToolCalls, error) {
	var (
		sb         strings.Builder
		gotMessage = false
		msg        = c.NewMessage(
			llms.WithRole(llms.RoleAssistant),
		)
		toolCalls = llms.ToolCalls{}
	)

	for _, item := range output {
		nextMsg, err := c.responseMessageFromOutputItem(msg, &sb, &gotMessage, item)
		if err != nil {
			return nil, nil, err
		}

		msg = nextMsg

		nextToolCalls, handled, err := c.responseToolCallsFromOutputItem(item)
		if err != nil {
			return nil, nil, err
		}

		if handled {
			toolCalls = append(toolCalls, nextToolCalls...)
		}
	}

	msg = msg.Update(llms.WithTextContent(sb.String()))

	return msg, toolCalls, nil
}

func (c *conversation) responseMessageFromOutputItem(
	msg *llms.Message,
	sb *strings.Builder,
	gotMessage *bool,
	item responses.ResponseOutputItemUnion,
) (*llms.Message, error) {
	providerMsg, ok := item.AsAny().(responses.ResponseOutputMessage)
	if !ok {
		if reasoning, ok := item.AsAny().(responses.ResponseReasoningItem); ok {
			slog.Debug("Got reasoning", "reasoning", reasoning.Summary)
		}

		return msg, nil
	}

	if *gotMessage {
		slog.Error("multiple assistant messages detected", "output_item", providerMsg)
		return nil, fmt.Errorf("got more than one message in output, not sure what to do with this")
	}

	msg = msg.Update(llms.WithID(providerMsg.ID))

	for _, contentItem := range providerMsg.Content {
		switch content := contentItem.AsAny().(type) {
		case responses.ResponseOutputText:
			sb.WriteString(content.Text)
		case responses.ResponseOutputRefusal:
			return nil, fmt.Errorf("%w: %s", llms.ErrPromptRefused, content.Refusal)
		}
	}

	*gotMessage = true

	return msg, nil
}

func (c *conversation) responseToolCallsFromOutputItem(item responses.ResponseOutputItemUnion) (llms.ToolCalls, bool, error) {
	toolCall, ok := item.AsAny().(responses.ResponseFunctionToolCall)
	if !ok {
		return nil, false, nil
	}

	args, err := c.Session().Tools.GetArgs(toolCall.Name, []byte(toolCall.Arguments))
	if err != nil {
		return nil, false, fmt.Errorf("failed to parse arguments for tool call to tool %q: %w", toolCall.Name, err)
	}

	return llms.ToolCalls{{
		ID:   toolCall.CallID,
		Name: toolCall.Name,
		Args: args,
	}}, true, nil
}
