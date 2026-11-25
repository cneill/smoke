package gemini //nolint:testpackage // Tests exercise provider conversion internals.

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/cneill/smoke/pkg/llms"
	"github.com/cneill/smoke/pkg/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/genai/interactions/models/interactions"
)

func TestStreamAccumulatorBuildsSteps(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		events   string
		wantText string
		check    func(t *testing.T, steps []interactions.Step)
	}{
		{
			name: "text deltas concatenate",
			events: `[
				{"event_type":"step.start","index":0,"step":{"type":"model_output"}},
				{"event_type":"step.delta","index":0,"delta":{"type":"text","text":"Hel"}},
				{"event_type":"step.delta","index":0,"delta":{"type":"text","text":"lo"}},
				{"event_type":"step.stop","index":0}
			]`,
			wantText: "Hello",
			check: func(t *testing.T, steps []interactions.Step) {
				t.Helper()
				require.Len(t, steps, 1)
				assert.Equal(t, []string{"Hello"}, contentTexts(steps[0].ModelOutputStep.Content))
			},
		},
		{
			name: "function call arguments assembled",
			events: `[
				{"event_type":"step.start","index":0,"step":{"type":"function_call","id":"call","name":"test_tool","arguments":{}}},
				{"event_type":"step.delta","index":0,"delta":{"type":"arguments_delta","arguments":"{\"path\":"}},
				{"event_type":"step.delta","index":0,"delta":{"type":"arguments_delta","arguments":"\"README.md\"}"}},
				{"event_type":"step.stop","index":0}
			]`,
			check: func(t *testing.T, steps []interactions.Step) {
				t.Helper()
				require.Len(t, steps, 1)
				assert.Equal(t, "call", steps[0].FunctionCallStep.ID)
				assert.Equal(t, map[string]any{"path": "README.md"}, steps[0].FunctionCallStep.Arguments)
			},
		},
		{
			name: "thought summary and signature applied",
			events: `[
				{"event_type":"step.start","index":0,"step":{"type":"thought"}},
				{"event_type":"step.delta","index":0,"delta":{"type":"thought_summary","content":{"type":"text","text":"think"}}},
				{"event_type":"step.delta","index":0,"delta":{"type":"thought_summary","content":{"type":"text","text":"ing"}}},
				{"event_type":"step.delta","index":0,"delta":{"type":"thought_signature","signature":"sig"}},
				{"event_type":"step.delta","index":0,"delta":{"type":"thought_signature"}},
				{"event_type":"step.start","index":1,"step":{"type":"model_output"}},
				{"event_type":"step.delta","index":1,"delta":{"type":"text","text":"answer"}}
			]`,
			wantText: "answer",
			check: func(t *testing.T, steps []interactions.Step) {
				t.Helper()
				require.Len(t, steps, 2)
				assert.Equal(t, "sig", utils.Deref(steps[0].ThoughtStep.Signature))
				require.Len(t, steps[0].ThoughtStep.Summary, 1)
				assert.Equal(t, "thinking", steps[0].ThoughtStep.Summary[0].TextContent.Text)
				assert.Equal(t, interactions.StepTypeModelOutput, steps[1].Type)
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			accumulator, text, err := accumulate(t, test.events)
			require.NoError(t, err)
			assert.Equal(t, test.wantText, text)
			test.check(t, accumulator.Steps())
		})
	}
}

func TestStreamAccumulatorCompletedSnapshotOverridesStreamedSteps(t *testing.T) {
	t.Parallel()

	accumulator, _, err := accumulate(t, `[
		{"event_type":"step.start","index":0,"step":{"type":"model_output"}},
		{"event_type":"step.delta","index":0,"delta":{"type":"text","text":"partial"}},
		{"event_type":"interaction.completed","interaction":{"id":"a","status":"completed",
			"steps":[{"type":"model_output","content":[{"type":"text","text":"full"}]}]}}
	]`)
	require.NoError(t, err)

	steps := accumulator.Steps()
	require.Len(t, steps, 1)
	assert.Equal(t, []string{"full"}, contentTexts(steps[0].ModelOutputStep.Content))
}

func TestStreamAccumulatorMultiHopKeepsPriorSteps(t *testing.T) {
	t.Parallel()

	accumulator, text, err := accumulate(t, `[
		{"event_type":"interaction.created","interaction":{"id":"hop1","status":"in_progress"}},
		{"event_type":"step.start","index":0,"step":{"type":"model_output"}},
		{"event_type":"step.delta","index":0,"delta":{"type":"text","text":"first "}},
		{"event_type":"interaction.completed","interaction":{"id":"hop1","status":"requires_continuation"}},
		{"event_type":"interaction.created","interaction":{"id":"hop2","status":"in_progress"}},
		{"event_type":"step.start","index":0,"step":{"type":"model_output"}},
		{"event_type":"step.delta","index":0,"delta":{"type":"text","text":"second"}},
		{"event_type":"interaction.completed","interaction":{"id":"hop2","status":"completed",
			"usage":{"total_input_tokens":7}}}
	]`)
	require.NoError(t, err)

	assert.Equal(t, "first second", text)
	assert.Equal(t, "hop2", accumulator.id)
	assert.Equal(t, interactions.InteractionStatusCompleted, accumulator.status)
	assert.Equal(t, 7, utils.Deref(accumulator.usage.TotalInputTokens))

	steps := accumulator.Steps()
	require.Len(t, steps, 2)
	assert.Equal(t, []string{"first "}, contentTexts(steps[0].ModelOutputStep.Content))
	assert.Equal(t, []string{"second"}, contentTexts(steps[1].ModelOutputStep.Content))
}

func TestStreamAccumulatorInvalidStreamedArgumentsRecordArgsError(t *testing.T) {
	t.Parallel()

	accumulator, _, err := accumulate(t, `[
		{"event_type":"step.start","index":0,"step":{"type":"function_call","id":"call","name":"test_tool","arguments":{}}},
		{"event_type":"step.delta","index":0,"delta":{"type":"arguments_delta","arguments":"{\"path\":"}},
		{"event_type":"step.stop","index":0}
	]`)
	require.NoError(t, err)
	assert.Equal(t, `{"path":`, accumulator.rawArgs["call"])

	conv := testConversation(t)
	msg, err := conv.stepsToMessage("interaction", accumulator.Steps(), accumulator.rawArgs)
	require.NoError(t, err)

	calls := msg.ToolCalls()
	require.Len(t, calls, 1)
	assert.Equal(t, `{"path":`, calls[0].RawArgs)
	assert.NotEmpty(t, calls[0].ArgsError)
}

func TestStreamAccumulatorNilPayloadsDoNotPanic(t *testing.T) {
	t.Parallel()

	accumulator := newStreamAccumulator()
	events := []interactions.InteractionSSEEvent{
		{Type: interactions.InteractionSSEEventTypeInteractionCreated},
		{Type: interactions.InteractionSSEEventTypeStepStart},
		{Type: interactions.InteractionSSEEventTypeStepDelta},
		{Type: interactions.InteractionSSEEventTypeStepStop},
		{Type: interactions.InteractionSSEEventTypeInteractionStatusUpdate},
		{Type: interactions.InteractionSSEEventTypeInteractionCompleted},
		{
			Type:      interactions.InteractionSSEEventTypeStepDelta,
			StepDelta: &interactions.StepDelta{Index: 0, Delta: interactions.StepDeltaData{Type: interactions.StepDeltaDataTypeText}},
		},
	}

	for _, event := range events {
		text, err := accumulator.Handle(event)
		require.NoError(t, err)
		assert.Empty(t, text)
	}

	_, err := accumulator.Handle(interactions.InteractionSSEEvent{Type: interactions.InteractionSSEEventTypeError})
	require.ErrorIs(t, err, llms.ErrCompletion)
}

func TestStreamAccumulatorErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		events  string
		wantMsg string
	}{
		{
			name:    "error event",
			events:  `[{"event_type":"error","error":{"code":"RESOURCE_EXHAUSTED","message":"slow down"}}]`,
			wantMsg: "slow down",
		},
		{
			name: "arguments for non-function step",
			events: `[
				{"event_type":"step.start","index":0,"step":{"type":"model_output"}},
				{"event_type":"step.delta","index":0,"delta":{"type":"arguments_delta","arguments":"{}"}},
				{"event_type":"step.stop","index":0}
			]`,
			wantMsg: "non-function step",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			_, _, err := accumulate(t, test.events)
			require.ErrorIs(t, err, llms.ErrCompletion)
			assert.ErrorContains(t, err, test.wantMsg)
		})
	}
}

// accumulate feeds JSON-encoded SSE events through a new accumulator, returning the concatenated text deltas and the
// first error encountered.
func accumulate(t *testing.T, raw string) (*streamAccumulator, string, error) {
	t.Helper()

	var events []interactions.InteractionSSEEvent

	require.NoError(t, json.Unmarshal([]byte(raw), &events))

	accumulator := newStreamAccumulator()

	var text strings.Builder

	for _, event := range events {
		delta, err := accumulator.Handle(event)
		if err != nil {
			return accumulator, text.String(), err
		}

		text.WriteString(delta)
	}

	return accumulator, text.String(), nil
}
