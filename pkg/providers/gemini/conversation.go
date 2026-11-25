package gemini

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"github.com/cneill/smoke/pkg/llms"
	"github.com/cneill/smoke/pkg/providers/base"
	"github.com/cneill/smoke/pkg/tools"
	"github.com/cneill/smoke/pkg/utils"
	genai "google.golang.org/genai/interactions"
	"google.golang.org/genai/interactions/models/interactions"
	"google.golang.org/genai/interactions/models/operations"
	"google.golang.org/genai/interactions/retry"
)

const (
	// noToolOutput replaces empty tool results so the model always sees an explicit result.
	noToolOutput = "[no output]"

	// retryStrategy is the documented retry.Config.Strategy value for exponential backoff capped at MaxRetries. The
	// retry package does not export constants for its strategies, so this is the one SDK string defined locally.
	retryStrategy = "attempt-count-backoff"

	// Retry settings mirror the 5-retry policy used by the other providers. Intervals are in milliseconds.
	maxRetries             = 5
	retryInitialIntervalMS = 500
	retryMaxIntervalMS     = 60_000
	retryMaxElapsedMS      = 300_000
	retryBackoffExponent   = 1.5

	// messageIDLength matches the random ID length llms.NewMessage uses by default.
	messageIDLength = 16
)

type conversation struct {
	*base.Conversation

	client *genai.GenAI
}

// interactionResult is the provider-agnostic subset of a finished interaction shared by both send paths.
type interactionResult struct {
	// interactionID is the server's ID, used only for logging. It is not a usable message ID: with Store=false the
	// server may omit it, and automatic continuation gives each hop of a single turn a different one.
	interactionID string
	status        interactions.InteractionStatus
	usage         *interactions.Usage
	steps         []interactions.Step
	errors        []interactions.Error
	// rawArgs preserves unparseable streamed function-call argument JSON keyed by call ID so newToolCall can attach
	// ArgsError instead of aborting the turn.
	rawArgs map[string]string
}

func retryOption() operations.Option {
	return operations.WithRetries(retry.Config{
		Strategy:              retryStrategy,
		MaxRetries:            new(maxRetries),
		RetryConnectionErrors: true,
		Backoff: &retry.BackoffStrategy{
			InitialInterval: retryInitialIntervalMS,
			MaxInterval:     retryMaxIntervalMS,
			Exponent:        retryBackoffExponent,
			MaxElapsedTime:  retryMaxElapsedMS,
		},
	})
}

// sendNoStream relies on the SDK's default automatic continuation, which merges all hops into one Interaction.
func (c *conversation) sendNoStream(ctx context.Context) error {
	resp, err := c.client.Interactions.Create(ctx, c.getCreateParams(false), retryOption())
	if err != nil {
		return fmt.Errorf("%w: %w", llms.ErrCompletion, err)
	}

	interaction := resp.Interaction
	if interaction == nil {
		return fmt.Errorf("%w: no interaction returned", llms.ErrEmptyResponse)
	}

	return c.handleResult(ctx, utils.RandID(messageIDLength), interactionResult{
		interactionID: utils.Deref(interaction.ID),
		status:        interaction.Status,
		usage:         interaction.Usage,
		steps:         interaction.Steps,
		errors:        interaction.Errors,
	})
}

func (c *conversation) sendStream(ctx context.Context) error {
	resp, err := c.client.Interactions.Create(ctx, c.getCreateParams(true), retryOption())
	if err != nil {
		return fmt.Errorf("%w: %w", llms.ErrCompletion, err)
	}

	events := resp.InteractionSSEStreamEvent
	if events == nil {
		return fmt.Errorf("%w: no event stream returned", llms.ErrEmptyResponse)
	}

	defer func() {
		if err := events.Close(); err != nil {
			slog.Debug("failed to close Interactions event stream", "error", err)
		}
	}()

	// One local ID per turn ties every text delta to the final message, which the history view relies on to replace
	// the streaming placeholder. See interactionResult.interactionID for why the server ID can't be used.
	messageID := utils.RandID(messageIDLength)
	accumulator := newStreamAccumulator()

	for events.Next() {
		event := events.Value()
		if event == nil {
			continue
		}

		text, err := accumulator.Handle(event.Data)
		if err != nil {
			return err
		}

		if text != "" {
			c.Emit(ctx, llms.EventTextDelta{ID: messageID, Text: text})
		}
	}

	if err := events.Err(); err != nil {
		return fmt.Errorf("%w: streaming: %w", llms.ErrCompletion, err)
	}

	return c.handleResult(ctx, messageID, interactionResult{
		interactionID: accumulator.id,
		status:        accumulator.status,
		usage:         accumulator.usage,
		steps:         accumulator.Steps(),
		rawArgs:       accumulator.rawArgs,
	})
}

// getCreateParams builds a stateless request (Store=false) carrying the full session history. Everything here must be
// derived deterministically from the session so that an unchanged prefix serializes identically between turns, which
// is what Gemini's implicit prefix caching keys on. Temperature and CachedContent are deliberately left unset.
func (c *conversation) getCreateParams(stream bool) operations.CreateInteractionRequest {
	session := c.Session()
	config := c.Config()

	body := interactions.CreateModelInteraction{
		Model:  interactions.Model(config.ModelInfo.Model),
		Input:  new(interactions.NewInteractionsInput(c.sessionSteps(session))),
		Tools:  c.functionTools(session),
		Store:  new(false),
		Stream: new(stream),
		GenerationConfig: &interactions.GenerationConfig{
			MaxOutputTokens:   new(int(config.MaxTokens)),
			ThinkingSummaries: interactions.ThinkingSummariesAuto.ToPointer(),
		},
	}

	if session.SystemMessage != "" {
		body.SystemInstruction = new(session.SystemMessage)
	}

	if config.Effort != "" {
		body.GenerationConfig.ThinkingLevel = interactions.ThinkingLevel(config.Effort).ToPointer()
	}

	return operations.CreateInteractionRequest{
		Body: operations.NewCreateInteractionRequestBody(body),
	}
}

func (c *conversation) sessionSteps(session *llms.Session) []interactions.Step {
	steps := make([]interactions.Step, 0, len(session.Messages))

	for _, msg := range session.Messages {
		switch msg.Role {
		case llms.RoleUser:
			steps = append(steps, interactions.NewStep(interactions.UserInputStep{
				Content: []interactions.Content{
					interactions.NewContent(interactions.TextContent{Text: msg.TextContent()}),
				},
			}))
		case llms.RoleSystem:
			// Sent as SystemInstruction instead.
		case llms.RoleAssistant:
			steps = append(steps, assistantSteps(msg)...)
		case llms.RoleTool:
			toolCalls := msg.ToolCalls()
			if n := len(toolCalls); n != 1 {
				slog.Warn(
					"got wrong number of tool calls referenced in message with tool role (expecting 1); skipping",
					"num", n, "names", toolCalls.Names(),
				)

				continue
			}

			steps = append(steps, toolResultStep(msg, toolCalls[0]))
		case llms.RoleUnknown:
			slog.Warn("got message with unknown role", "message", msg.TextContent())
		}
	}

	return steps
}

// assistantSteps converts an assistant message into steps, preserving block order. Consecutive text blocks are merged
// into a single model_output step. Reasoning from other providers cannot be replayed and is dropped.
func assistantSteps(msg *llms.Message) []interactions.Step {
	var (
		steps   []interactions.Step
		pending []interactions.Content
	)

	flush := func() {
		if len(pending) > 0 {
			steps = append(steps, interactions.NewStep(interactions.ModelOutputStep{Content: pending}))
			pending = nil
		}
	}

	for _, block := range msg.Blocks {
		switch block.Type() { //nolint:exhaustive // Images are never produced by the assistant.
		case llms.BlockTypeText:
			if strings.TrimSpace(block.Text.Text) != "" {
				pending = append(pending, interactions.NewContent(interactions.TextContent{Text: block.Text.Text}))
			}
		case llms.BlockTypeToolCall:
			flush()

			steps = append(steps, interactions.NewStep(functionCallStep(block.ToolCall.ToolCall)))
		case llms.BlockTypeReasoning:
			detail := block.Reasoning.Detail
			if detail.Type() != llms.ReasoningDetailTypeGemini || detail.Gemini.Source != llms.LLMTypeGemini {
				continue
			}

			flush()

			steps = append(steps, interactions.NewStep(thoughtStep(block.Reasoning)))
		}
	}

	flush()

	return steps
}

func functionCallStep(call llms.ToolCall) interactions.FunctionCallStep {
	args, err := utils.DecodeJSONObject([]byte(call.ArgsString()))
	if err != nil {
		slog.Warn("failed to decode tool call arguments for replay, sending empty arguments",
			"tool_name", call.Name, "error", err)

		args = map[string]any{}
	}

	return interactions.FunctionCallStep{
		ID:        call.ID,
		Name:      call.Name,
		Arguments: args,
	}
}

func thoughtStep(reasoning *llms.ReasoningBlock) interactions.ThoughtStep {
	thought := interactions.ThoughtStep{}

	if signature := reasoning.Detail.Gemini.Signature; signature != "" {
		thought.Signature = new(signature)
	}

	for _, text := range reasoning.Summary {
		thought.Summary = append(thought.Summary,
			interactions.NewThoughtSummaryContent(interactions.TextContent{Text: text}))
	}

	return thought
}

func toolResultStep(msg *llms.Message, call llms.ToolCall) interactions.Step {
	var result interactions.FunctionResultStepResultUnion

	if images := msg.Images(); len(images) > 0 {
		parts := make([]interactions.FunctionResultSubcontent, 0, len(images)+1)
		if text := msg.TextContent(); text != "" {
			parts = append(parts, interactions.NewFunctionResultSubcontent(interactions.TextContent{Text: text}))
		}

		for _, image := range images {
			parts = append(parts, interactions.NewFunctionResultSubcontent(interactions.ImageContent{
				Data:     new(base64.StdEncoding.EncodeToString(image.Data)),
				MimeType: interactions.ImageContentMimeType(image.MediaType).ToPointer(),
			}))
		}

		result = interactions.NewFunctionResultStepResultUnion(parts)
	} else {
		text := msg.TextContent()
		if text == "" {
			text = noToolOutput
		}

		result = interactions.NewFunctionResultStepResultUnion(text)
	}

	return interactions.NewStep(interactions.FunctionResultStep{
		CallID:  call.ID,
		Name:    new(call.Name),
		IsError: new(msg.Error != ""),
		Result:  result,
	})
}

func (c *conversation) functionTools(session *llms.Session) []interactions.Tool {
	sessionTools := session.Tools.GetTools()
	results := make([]interactions.Tool, 0, len(sessionTools))

	for _, tool := range sessionTools {
		params := tool.Params()

		properties, err := params.JSONSchemaProperties()
		if err != nil {
			slog.Error("failed to get JSON Schema properties for tool, skipping", "tool_name", tool.Name(), "error", err)
			continue
		}

		results = append(results, interactions.NewTool(interactions.Function{
			Name:        new(tool.Name()),
			Description: new(tool.Description()),
			Parameters: map[string]any{
				"type":       tools.ParamTypeObject,
				"properties": properties,
				"required":   params.RequiredKeys(),
			},
		}))
	}

	return results
}

// handleResult is the shared tail of both send paths: report usage, validate status, convert steps, and emit.
func (c *conversation) handleResult(ctx context.Context, messageID string, result interactionResult) error {
	if result.usage != nil {
		slog.Debug("gemini usage", "interaction_id", result.interactionID,
			"cached_tokens", utils.Deref(result.usage.TotalCachedTokens))
		c.Emit(ctx, usageEvent(result.usage))
	}

	if err := checkStatus(result.status, result.errors); err != nil {
		return err
	}

	if len(result.steps) == 0 {
		return fmt.Errorf("%w: no steps returned", llms.ErrEmptyResponse)
	}

	msg, err := c.stepsToMessage(messageID, result.steps, result.rawArgs)
	if err != nil {
		return err
	}

	if len(msg.Blocks) == 0 {
		return fmt.Errorf("%w: no usable content in steps", llms.ErrEmptyResponse)
	}

	c.finish(ctx, msg)

	return nil
}

// usageEvent converts Gemini usage into a Smoke usage event. It assumes TotalInputTokens already includes cached
// tokens and that TotalOutputTokens excludes thought tokens (as candidatesTokenCount does in generateContent); adjust
// here if live numbers look off.
func usageEvent(usage *interactions.Usage) llms.EventUsageUpdate {
	if usage == nil {
		return llms.EventUsageUpdate{}
	}

	return llms.EventUsageUpdate{
		InputTokens:  int64(utils.Deref(usage.TotalInputTokens)),
		OutputTokens: int64(utils.Deref(usage.TotalOutputTokens) + utils.Deref(usage.TotalThoughtTokens)),
	}
}

func checkStatus(status interactions.InteractionStatus, errs []interactions.Error) error {
	switch status {
	case interactions.InteractionStatusCompleted, interactions.InteractionStatusRequiresAction:
		return nil
	case interactions.InteractionStatusIncomplete, interactions.InteractionStatusBudgetExceeded:
		return fmt.Errorf("%w: interaction status %q", llms.ErrMaxTokens, status)
	case interactions.InteractionStatusFailed:
		if len(errs) == 0 {
			return fmt.Errorf("%w: interaction failed", llms.ErrCompletion)
		}

		messages := make([]string, len(errs))
		for i := range errs {
			messages[i] = errorMessage(&errs[i])
		}

		return fmt.Errorf("%w: interaction failed: %s", llms.ErrCompletion, strings.Join(messages, "; "))
	case interactions.InteractionStatusCancelled, interactions.InteractionStatusRequiresContinuation,
		interactions.InteractionStatusInProgress, interactions.InteractionStatusQueued:
		return fmt.Errorf("%w: unexpected interaction status %q", llms.ErrCompletion, status)
	}

	return fmt.Errorf("%w: unknown interaction status %q", llms.ErrCompletion, status)
}

func (c *conversation) stepsToMessage(
	messageID string, steps []interactions.Step, rawArgs map[string]string,
) (*llms.Message, error) {
	blocks := make([]llms.Block, 0, len(steps))

	for _, step := range steps {
		switch step.Type { //nolint:exhaustive // Input echoes and server-side tool steps are not surfaced.
		case interactions.StepTypeThought:
			blocks = append(blocks, reasoningBlock(step.ThoughtStep))
		case interactions.StepTypeModelOutput:
			output := step.ModelOutputStep
			if output.Error != nil {
				return nil, fmt.Errorf("%w: model output error: %s", llms.ErrCompletion, statusMessage(output.Error))
			}

			for _, content := range output.Content {
				if content.Type == interactions.ContentTypeText && strings.TrimSpace(content.TextContent.Text) != "" {
					blocks = append(blocks, llms.TextContentBlock(content.TextContent.Text))
				}
			}
		case interactions.StepTypeFunctionCall:
			call := step.FunctionCallStep
			blocks = append(blocks, llms.ToolContentBlock(c.newToolCall(call.ID, call.Name, rawToolArgs(call, rawArgs))))
		default:
			slog.Debug("ignoring interaction step", "type", step.Type)
		}
	}

	return c.NewMessage(
		llms.WithID(messageID),
		llms.WithRole(llms.RoleAssistant),
		llms.WithBlocks(blocks...),
	), nil
}

func reasoningBlock(thought *interactions.ThoughtStep) llms.Block {
	var summary []string

	for _, part := range thought.Summary {
		if part.Type == interactions.ThoughtSummaryContentTypeText {
			summary = append(summary, part.TextContent.Text)
		}
	}

	return llms.ReasoningContentBlock(llms.ReasoningBlock{
		Summary: summary,
		Detail: llms.ReasoningDetail{
			Gemini: &llms.GeminiReasoningDetail{
				Source:    llms.LLMTypeGemini,
				Signature: utils.Deref(thought.Signature),
			},
		},
	})
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

func (c *conversation) finish(ctx context.Context, msg *llms.Message) {
	if msg.HasToolCalls() {
		c.HasPendingToolCalls = true
		c.Emit(ctx, llms.EventToolCallsRequested{Message: msg})

		return
	}

	c.HasPendingToolCalls = false
	c.Emit(ctx, llms.EventFinalMessage{Message: msg})
}

func rawToolArgs(call *interactions.FunctionCallStep, rawArgs map[string]string) string {
	if raw, ok := rawArgs[call.ID]; ok {
		return raw
	}

	return encodeArgs(call.Arguments)
}

func encodeArgs(args map[string]any) string {
	if args == nil {
		return "{}"
	}

	encoded, err := json.Marshal(args)
	if err != nil {
		slog.Warn("failed to encode function call arguments", "error", err)
		return "{}"
	}

	return string(encoded)
}

func statusMessage(status *interactions.Status) string {
	switch {
	case status == nil:
		return "unknown error"
	case status.Message != nil && status.Code != nil:
		return fmt.Sprintf("%s (code %d)", *status.Message, *status.Code)
	case status.Message != nil:
		return *status.Message
	case status.Code != nil:
		return fmt.Sprintf("code %d", *status.Code)
	}

	return "unknown error"
}

func errorMessage(err *interactions.Error) string {
	switch {
	case err == nil:
		return "unknown error"
	case err.Message != nil && err.Code != nil:
		return fmt.Sprintf("%s (%s)", *err.Message, *err.Code)
	case err.Message != nil:
		return *err.Message
	case err.Code != nil:
		return *err.Code
	}

	return "unknown error"
}
