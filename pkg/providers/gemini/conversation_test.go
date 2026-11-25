package gemini //nolint:testpackage // Tests exercise provider conversion internals.

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/cneill/smoke/pkg/llmctx/modes"
	"github.com/cneill/smoke/pkg/llms"
	"github.com/cneill/smoke/pkg/providers/base"
	"github.com/cneill/smoke/pkg/tools"
	"github.com/cneill/smoke/pkg/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/genai/interactions/models/interactions"
)

type testTool struct{}

func (testTool) Name() string             { return "test_tool" }
func (testTool) Description() string      { return "test tool" }
func (testTool) Examples() tools.Examples { return nil }
func (testTool) Run(context.Context, tools.Args) (*tools.Output, error) {
	return &tools.Output{Text: "ok"}, nil
}

func (testTool) Params() tools.Params {
	return tools.Params{{Key: "path", Type: tools.ParamTypeString, Required: true}}
}

func TestSessionStepsMapsRolesInOrder(t *testing.T) {
	t.Parallel()

	conv := testConversation(t)
	call := llms.ToolCall{ID: "call", Name: "test_tool", RawArgs: `{"path":"README.md"}`}
	conv.Session().Messages = []*llms.Message{
		llms.NewMessage(llms.WithRole(llms.RoleUser), llms.WithBlocks(llms.TextContentBlock("question"))),
		llms.NewMessage(llms.WithRole(llms.RoleSystem), llms.WithBlocks(llms.TextContentBlock("ignored"))),
		llms.NewMessage(llms.WithRole(llms.RoleAssistant), llms.WithBlocks(
			geminiReasoningBlock("thinking", "signature"),
			llms.TextContentBlock("a"),
			llms.TextContentBlock("b"),
			otherProviderReasoningBlock(),
			openAIReasoningBlock(),
			llms.TextContentBlock("  "),
			llms.TextContentBlock("c"),
			llms.ToolContentBlock(call),
		)),
		llms.NewMessage(llms.WithRole(llms.RoleTool), llms.WithTextContent("result"), llms.WithToolCalls(call)),
	}

	steps := conv.sessionSteps(conv.Session())
	assert.Equal(t, []interactions.StepType{
		interactions.StepTypeUserInput,
		interactions.StepTypeThought,
		interactions.StepTypeModelOutput,
		interactions.StepTypeFunctionCall,
		interactions.StepTypeFunctionResult,
	}, stepTypes(steps))

	assert.Equal(t, "question", steps[0].UserInputStep.Content[0].TextContent.Text)
	assert.Equal(t, "signature", utils.Deref(steps[1].ThoughtStep.Signature))
	assert.Equal(t, "thinking", steps[1].ThoughtStep.Summary[0].TextContent.Text)
	// Blank text is dropped and incompatible reasoning does not split the merged model output.
	assert.Equal(t, []string{"a", "b", "c"}, contentTexts(steps[2].ModelOutputStep.Content))
	assert.Equal(t, "call", steps[3].FunctionCallStep.ID)
	assert.Equal(t, map[string]any{"path": "README.md"}, steps[3].FunctionCallStep.Arguments)
	assert.Equal(t, "call", steps[4].FunctionResultStep.CallID)
	assert.Equal(t, "result", utils.Deref(steps[4].FunctionResultStep.Result.Str))
}

func TestSessionStepsSkipsMalformedToolMessages(t *testing.T) {
	t.Parallel()

	conv := testConversation(t)
	conv.Session().Messages = []*llms.Message{
		llms.NewMessage(llms.WithRole(llms.RoleTool), llms.WithBlocks(llms.TextContentBlock("orphan"))),
	}

	assert.Empty(t, conv.sessionSteps(conv.Session()))
}

func TestToolResultStep(t *testing.T) {
	t.Parallel()

	call := llms.ToolCall{ID: "call", Name: "test_tool"}

	tests := []struct {
		name      string
		blocks    []llms.Block
		errText   string
		wantText  string
		wantImage bool
		wantError bool
	}{
		{name: "text", blocks: []llms.Block{llms.TextContentBlock("output")}, wantText: "output"},
		{name: "empty", wantText: noToolOutput},
		{name: "error", blocks: []llms.Block{llms.TextContentBlock("bad")}, errText: "boom", wantText: "bad", wantError: true},
		{name: "image", blocks: []llms.Block{llms.ImageContentBlock([]byte{1, 2}, "image/png")}, wantImage: true},
		{
			name: "text and image",
			blocks: []llms.Block{
				llms.TextContentBlock("caption"),
				llms.ImageContentBlock([]byte{1, 2}, "image/png"),
			},
			wantText:  "caption",
			wantImage: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			msg := llms.NewMessage(llms.WithRole(llms.RoleTool), llms.WithBlocks(test.blocks...), llms.WithToolCalls(call))
			msg.Error = test.errText

			step := toolResultStep(msg, call)
			require.Equal(t, interactions.StepTypeFunctionResult, step.Type)

			result := step.FunctionResultStep
			assert.Equal(t, "call", result.CallID)
			assert.Equal(t, "test_tool", utils.Deref(result.Name))
			assert.Equal(t, test.wantError, utils.Deref(result.IsError))

			if !test.wantImage {
				assert.Equal(t, test.wantText, utils.Deref(result.Result.Str))
				return
			}

			subcontents := result.Result.ArrayOfFunctionResultSubcontent
			if test.wantText != "" {
				require.Len(t, subcontents, 2)
				require.NotNil(t, subcontents[0].TextContent)
				assert.Equal(t, test.wantText, subcontents[0].TextContent.Text)
				subcontents = subcontents[1:]
			}

			require.Len(t, subcontents, 1)
			image := subcontents[0].ImageContent
			require.NotNil(t, image)
			assert.Equal(t, "AQI=", utils.Deref(image.Data))
			assert.Equal(t, interactions.ImageContentMimeTypeImagePng, utils.Deref(image.MimeType))
		})
	}
}

func TestStepsToMessagePreservesOrderAndRecordsArgsErrors(t *testing.T) {
	t.Parallel()

	conv := testConversation(t)
	steps := parseSteps(t, `[
		{"type":"thought","signature":"sig","summary":[{"type":"text","text":"thinking"}]},
		{"type":"model_output","content":[{"type":"text","text":"first"}]},
		{"type":"function_call","id":"good","name":"test_tool","arguments":{"path":"README.md"}},
		{"type":"user_input","content":[{"type":"text","text":"echo"}]},
		{"type":"model_output","content":[{"type":"text","text":"second"}]},
		{"type":"function_call","id":"bad","name":"test_tool","arguments":{"other":true}}
	]`)

	msg, err := conv.stepsToMessage("interaction", steps, nil)
	require.NoError(t, err)
	assert.Equal(t, "interaction", msg.ID)
	assert.Equal(t, []llms.BlockType{
		llms.BlockTypeReasoning,
		llms.BlockTypeText,
		llms.BlockTypeToolCall,
		llms.BlockTypeText,
		llms.BlockTypeToolCall,
	}, blockTypes(msg.Blocks))

	detail := msg.Blocks[0].Reasoning.Detail
	assert.Equal(t, llms.ReasoningDetailTypeGemini, detail.Type())
	assert.Equal(t, "sig", detail.Gemini.Signature)
	assert.Equal(t, []string{"thinking"}, msg.Blocks[0].Reasoning.Summary)

	calls := msg.ToolCalls()
	require.Len(t, calls, 2)
	assert.Empty(t, calls[0].ArgsError)
	assert.JSONEq(t, `{"path":"README.md"}`, calls[0].RawArgs)
	assert.JSONEq(t, `{"other":true}`, calls[1].RawArgs)
	assert.NotEmpty(t, calls[1].ArgsError)
}

func TestStepsToMessageModelOutputError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		steps   string
		wantMsg string
	}{
		{
			name:    "message and code",
			steps:   `[{"type":"model_output","error":{"code":3,"message":"blocked"}}]`,
			wantMsg: "blocked (code 3)",
		},
		{
			name:    "code only",
			steps:   `[{"type":"model_output","error":{"code":7}}]`,
			wantMsg: "code 7",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			conv := testConversation(t)
			steps := parseSteps(t, test.steps)

			_, err := conv.stepsToMessage("interaction", steps, nil)
			require.ErrorIs(t, err, llms.ErrCompletion)
			assert.ErrorContains(t, err, test.wantMsg)
		})
	}
}

func TestGeminiReasoningRoundTrips(t *testing.T) {
	t.Parallel()

	conv := testConversation(t)
	steps := parseSteps(t, `[{"type":"thought","signature":"sig","summary":[{"type":"text","text":"why"}]}]`)

	msg, err := conv.stepsToMessage("interaction", steps, nil)
	require.NoError(t, err)

	replayed := assistantSteps(msg)
	require.Len(t, replayed, 1)
	assert.Equal(t, "sig", utils.Deref(replayed[0].ThoughtStep.Signature))
	assert.Equal(t, "why", replayed[0].ThoughtStep.Summary[0].TextContent.Text)
}

func TestCheckStatus(t *testing.T) {
	t.Parallel()

	failure := interactions.Error{Message: new("quota exhausted")}

	tests := []struct {
		status  interactions.InteractionStatus
		errs    []interactions.Error
		wantErr error
		wantMsg string
	}{
		{status: interactions.InteractionStatusCompleted},
		{status: interactions.InteractionStatusRequiresAction},
		{status: interactions.InteractionStatusIncomplete, wantErr: llms.ErrMaxTokens},
		{status: interactions.InteractionStatusBudgetExceeded, wantErr: llms.ErrMaxTokens},
		{status: interactions.InteractionStatusFailed, wantErr: llms.ErrCompletion},
		{
			status:  interactions.InteractionStatusFailed,
			errs:    []interactions.Error{failure},
			wantErr: llms.ErrCompletion,
			wantMsg: "quota exhausted",
		},
		{status: interactions.InteractionStatusCancelled, wantErr: llms.ErrCompletion},
		{status: interactions.InteractionStatusRequiresContinuation, wantErr: llms.ErrCompletion},
		{status: interactions.InteractionStatusInProgress, wantErr: llms.ErrCompletion},
		{status: interactions.InteractionStatusQueued, wantErr: llms.ErrCompletion},
		{status: "", wantErr: llms.ErrCompletion, wantMsg: "unknown"},
	}

	for _, test := range tests {
		t.Run(string(test.status)+test.wantMsg, func(t *testing.T) {
			t.Parallel()

			err := checkStatus(test.status, test.errs)
			if test.wantErr == nil {
				require.NoError(t, err)
				return
			}

			require.ErrorIs(t, err, test.wantErr)

			if test.wantMsg != "" {
				assert.ErrorContains(t, err, test.wantMsg)
			}
		})
	}
}

func TestUsageEvent(t *testing.T) {
	t.Parallel()

	assert.Equal(t, llms.EventUsageUpdate{}, usageEvent(nil))
	assert.Equal(t, llms.EventUsageUpdate{}, usageEvent(&interactions.Usage{}))
	assert.Equal(t, llms.EventUsageUpdate{InputTokens: 100, OutputTokens: 30}, usageEvent(&interactions.Usage{
		TotalInputTokens:   new(100),
		TotalOutputTokens:  new(10),
		TotalThoughtTokens: new(20),
	}))
}

func TestHandleResult(t *testing.T) {
	t.Parallel()

	usage := &interactions.Usage{TotalInputTokens: new(5)}
	toolSteps := parseSteps(t, `[{"type":"function_call","id":"call","name":"test_tool","arguments":{"path":"x"}}]`)
	textSteps := parseSteps(t, `[{"type":"model_output","content":[{"type":"text","text":"done"}]}]`)
	filteredSteps := parseSteps(t, `[
		{"type":"user_input","content":[{"type":"text","text":"echo"}]},
		{"type":"model_output","content":[{"type":"text","text":"   "}]}
	]`)

	tests := []struct {
		name        string
		result      interactionResult
		wantErr     error
		wantPending bool
		wantFinal   bool
	}{
		{
			name:        "tool calls",
			result:      interactionResult{status: interactions.InteractionStatusRequiresAction, usage: usage, steps: toolSteps},
			wantPending: true,
		},
		{
			name:      "final message",
			result:    interactionResult{status: interactions.InteractionStatusCompleted, usage: usage, steps: textSteps},
			wantFinal: true,
		},
		{
			name:    "no steps",
			result:  interactionResult{status: interactions.InteractionStatusCompleted, usage: usage},
			wantErr: llms.ErrEmptyResponse,
		},
		{
			name:    "only filtered steps",
			result:  interactionResult{status: interactions.InteractionStatusCompleted, usage: usage, steps: filteredSteps},
			wantErr: llms.ErrEmptyResponse,
		},
		{
			name:    "max tokens",
			result:  interactionResult{status: interactions.InteractionStatusIncomplete, usage: usage, steps: textSteps},
			wantErr: llms.ErrMaxTokens,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			conv := testConversation(t)
			events, err := handleResultEvents(t, conv, test.result)
			require.NotEmpty(t, events)
			assert.Equal(t, llms.EventUsageUpdate{InputTokens: 5}, events[0])

			if test.wantErr != nil {
				require.ErrorIs(t, err, test.wantErr)
				return
			}

			require.NoError(t, err)
			require.Len(t, events, 2)
			assert.Equal(t, test.wantPending, conv.HasPendingToolCalls)

			final, isFinal := events[1].(llms.EventFinalMessage)
			requested, isRequested := events[1].(llms.EventToolCallsRequested)

			assert.Equal(t, test.wantFinal, isFinal)
			assert.Equal(t, test.wantPending, isRequested)

			msg := final.Message
			if isRequested {
				msg = requested.Message
			}

			assert.Equal(t, "message", msg.ID)
			require.NoError(t, msg.OK())
		})
	}
}

// Implicit prefix caching only hits if an unchanged session serializes to identical bytes on every request.
func TestGetCreateParamsIsDeterministic(t *testing.T) {
	t.Parallel()

	conv := testConversation(t)
	call := llms.ToolCall{ID: "call", Name: "test_tool", RawArgs: `{"path":"README.md","n":1.50}`}
	conv.Session().Messages = []*llms.Message{
		llms.NewMessage(llms.WithRole(llms.RoleUser), llms.WithBlocks(llms.TextContentBlock("question"))),
		llms.NewMessage(llms.WithRole(llms.RoleAssistant), llms.WithBlocks(
			geminiReasoningBlock("thinking", "sig"), llms.ToolContentBlock(call))),
		llms.NewMessage(llms.WithRole(llms.RoleTool), llms.WithTextContent("result"), llms.WithToolCalls(call)),
	}

	first, err := json.Marshal(conv.getCreateParams(true).Body)
	require.NoError(t, err)

	second, err := json.Marshal(conv.getCreateParams(true).Body)
	require.NoError(t, err)

	assert.Equal(t, string(first), string(second))
	assert.Contains(t, string(first), `"n":1.50`, "numbers should replay exactly as received")

	var body map[string]any
	require.NoError(t, json.Unmarshal(first, &body))
	assert.Equal(t, false, body["store"])
	assert.Equal(t, true, body["stream"])
	assert.Equal(t, "system", body["system_instruction"])
	assert.NotContains(t, body, "previous_interaction_id")
	assert.NotContains(t, body, "cached_content")

	generationConfig, ok := body["generation_config"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, string(interactions.ThinkingLevelMedium), generationConfig["thinking_level"])
	assert.Equal(t, string(interactions.ThinkingSummariesAuto), generationConfig["thinking_summaries"])
	assert.NotContains(t, generationConfig, "temperature")
}

func testConversation(t *testing.T) *conversation {
	t.Helper()

	manager, err := tools.NewManager(&tools.ManagerOpts{ProjectPath: t.TempDir(), SessionName: "test"})
	require.NoError(t, err)
	manager.SetTools(testTool{})

	session, err := llms.NewSession(&llms.SessionOpts{
		Name:          "test",
		SystemMessage: "system",
		Tools:         manager,
		Mode:          modes.ModeWork,
		Config: &llms.Config{
			ModelInfo: llms.ModelInfo{Provider: llms.LLMTypeGemini, Model: "model"},
			MaxTokens: 1024,
			Effort:    string(interactions.ThinkingLevelMedium),
		},
	})
	require.NoError(t, err)

	send := func(context.Context) error { return nil }
	baseConv, _, err := base.NewConversation(context.Background(), &base.ConversationOpts{
		Session:      session,
		LLMInfo:      &llms.LLMInfo{Type: llms.LLMTypeGemini, ModelName: "model"},
		Config:       session.Config,
		SendStream:   send,
		SendNoStream: send,
	})
	require.NoError(t, err)

	return &conversation{Conversation: baseConv}
}

// handleResultEvents runs handleResult with a fixed message ID and collects every event it emits.
func handleResultEvents(t *testing.T, conv *conversation, result interactionResult) ([]llms.Event, error) {
	t.Helper()

	return collectEvents(t, conv, func(ctx context.Context) error {
		return conv.handleResult(ctx, "message", result)
	})
}

// collectEvents runs send and collects every event it emits until it returns.
func collectEvents(t *testing.T, conv *conversation, send func(context.Context) error) ([]llms.Event, error) {
	t.Helper()

	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
	defer cancel()

	errChan := make(chan error, 1)
	go func() {
		errChan <- send(ctx)
	}()

	var events []llms.Event

	for {
		select {
		case event := <-conv.Events():
			events = append(events, event)
		case err := <-errChan:
			return events, err
		case <-ctx.Done():
			require.FailNow(t, "timed out waiting for conversation events")
		}
	}
}

func parseSteps(t *testing.T, raw string) []interactions.Step {
	t.Helper()

	var steps []interactions.Step

	require.NoError(t, json.Unmarshal([]byte(raw), &steps))

	return steps
}

func stepTypes(steps []interactions.Step) []interactions.StepType {
	result := make([]interactions.StepType, len(steps))
	for i := range steps {
		result[i] = steps[i].Type
	}

	return result
}

func contentTexts(contents []interactions.Content) []string {
	result := make([]string, 0, len(contents))
	for _, content := range contents {
		if content.Type == interactions.ContentTypeText {
			result = append(result, content.TextContent.Text)
		}
	}

	return result
}

func blockTypes(blocks []llms.Block) []llms.BlockType {
	result := make([]llms.BlockType, len(blocks))
	for i := range blocks {
		result[i] = blocks[i].Type()
	}

	return result
}

func geminiReasoningBlock(summary, signature string) llms.Block {
	return llms.ReasoningContentBlock(llms.ReasoningBlock{
		Summary: []string{summary},
		Detail: llms.ReasoningDetail{
			Gemini: &llms.GeminiReasoningDetail{Source: llms.LLMTypeGemini, Signature: signature},
		},
	})
}

func otherProviderReasoningBlock() llms.Block {
	return llms.ReasoningContentBlock(llms.ReasoningBlock{
		Summary: []string{"claude thinking"},
		Detail: llms.ReasoningDetail{
			Anthropic: &llms.AnthropicReasoningDetail{Source: llms.LLMTypeClaude, Signature: "anthropic"},
		},
	})
}

func openAIReasoningBlock() llms.Block {
	return llms.ReasoningContentBlock(llms.ReasoningBlock{
		Detail: llms.ReasoningDetail{
			OpenAI: &llms.OpenAIReasoningDetail{Source: llms.LLMTypeChatGPT, ID: "reason"},
		},
	})
}
