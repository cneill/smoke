package gemini

import (
	"fmt"
	"log/slog"
	"maps"
	"slices"
	"strings"

	"github.com/cneill/smoke/pkg/llms"
	"github.com/cneill/smoke/pkg/utils"
	"google.golang.org/genai/interactions/models/interactions"
)

// streamAccumulator rebuilds the final interaction state from Interactions API SSE events. It is kept free of any
// network concerns so it can be unit tested with synthetic events.
//
// With automatic continuation (the SDK default), one stream can carry several "hops", each starting with its own
// interaction.created event and restarting step indices at 0. Steps from finished hops are moved to completed so a
// new hop never clobbers them.
type streamAccumulator struct {
	id     string
	status interactions.InteractionStatus
	// usage holds the most recent usage report. Each hop re-sends the full conversation, so the latest hop's input
	// count is the best approximation of the current context size.
	usage *interactions.Usage

	completed []interactions.Step
	current   map[int]*interactions.Step
	args      map[int]*strings.Builder
	// rawArgs stores unparseable streamed argument JSON keyed by function call ID so the caller can surface it as
	// ToolCall.ArgsError instead of failing the turn.
	rawArgs map[string]string
}

func newStreamAccumulator() *streamAccumulator {
	return &streamAccumulator{
		current: map[int]*interactions.Step{},
		args:    map[int]*strings.Builder{},
		rawArgs: map[string]string{},
	}
}

// Handle applies a single stream event. It returns any model output text contained in the event so the caller can
// emit it as a text delta.
func (a *streamAccumulator) Handle(event interactions.InteractionSSEEvent) (string, error) {
	switch event.Type { //nolint:exhaustive // Unknown and future event types are logged and ignored.
	case interactions.InteractionSSEEventTypeInteractionCreated:
		a.startHop(event.InteractionCreatedEvent)
	case interactions.InteractionSSEEventTypeStepStart:
		a.startStep(event.StepStart)
	case interactions.InteractionSSEEventTypeStepDelta:
		return a.applyDelta(event.StepDelta), nil
	case interactions.InteractionSSEEventTypeStepStop:
		return "", a.stopStep(event.StepStop)
	case interactions.InteractionSSEEventTypeInteractionStatusUpdate:
		a.statusUpdate(event.InteractionStatusUpdate)
	case interactions.InteractionSSEEventTypeInteractionCompleted:
		a.complete(event.InteractionCompletedEvent)
	case interactions.InteractionSSEEventTypeError:
		return "", streamError(event.ErrorEvent)
	default:
		slog.Debug("ignoring unhandled Interactions stream event", "type", event.Type)
	}

	return "", nil
}

// Steps returns the steps from all finished hops followed by the current hop's steps in index order.
func (a *streamAccumulator) Steps() []interactions.Step {
	return append(slices.Clone(a.completed), a.currentSteps()...)
}

func (a *streamAccumulator) currentSteps() []interactions.Step {
	steps := make([]interactions.Step, 0, len(a.current))
	for _, idx := range slices.Sorted(maps.Keys(a.current)) {
		steps = append(steps, *a.current[idx])
	}

	return steps
}

func (a *streamAccumulator) startHop(event *interactions.InteractionCreatedEvent) {
	if event == nil {
		return
	}

	interaction := event.Interaction

	a.completed = append(a.completed, a.currentSteps()...)
	a.current = map[int]*interactions.Step{}
	a.args = map[int]*strings.Builder{}
	a.id = interaction.ID
	a.status = interactions.InteractionStatus(interaction.Status)
	a.setUsage(interaction.Usage)
}

func (a *streamAccumulator) startStep(event *interactions.StepStart) {
	if event == nil {
		return
	}

	step := event.Step
	a.current[event.Index] = &step
}

func (a *streamAccumulator) statusUpdate(update *interactions.InteractionStatusUpdate) {
	if update == nil {
		return
	}

	a.status = interactions.InteractionStatus(update.Status)
	a.setUsage(update.Usage)
}

func (a *streamAccumulator) complete(event *interactions.InteractionCompletedEvent) {
	if event == nil {
		return
	}

	interaction := event.Interaction
	if interaction.ID != "" {
		a.id = interaction.ID
	}

	a.status = interactions.InteractionStatus(interaction.Status)
	a.setUsage(interaction.Usage)

	// The server's snapshot of the hop is authoritative when present.
	if len(interaction.Steps) > 0 {
		a.current = make(map[int]*interactions.Step, len(interaction.Steps))
		for idx := range interaction.Steps {
			a.current[idx] = &interaction.Steps[idx]
		}

		a.args = map[int]*strings.Builder{}
	}
}

func streamError(event *interactions.ErrorEvent) error {
	var errPayload *interactions.Error
	if event != nil {
		errPayload = event.Error
	}

	return fmt.Errorf("%w: stream error: %s", llms.ErrCompletion, errorMessage(errPayload))
}

func (a *streamAccumulator) setUsage(usage *interactions.Usage) {
	if usage != nil {
		a.usage = usage
	}
}

// applyDelta merges a step delta into the step at its index and returns any model output text it carried.
func (a *streamAccumulator) applyDelta(delta *interactions.StepDelta) string {
	if delta == nil {
		return ""
	}

	data := delta.Delta

	switch data.Type { //nolint:exhaustive // Media and server-side tool deltas are not used by Smoke.
	case interactions.StepDeltaDataTypeText:
		return a.applyTextDelta(delta.Index, data.TextDelta)
	case interactions.StepDeltaDataTypeThoughtSummary:
		a.applyThoughtSummary(delta.Index, data.ThoughtSummaryDelta)
	case interactions.StepDeltaDataTypeThoughtSignature:
		a.applyThoughtSignature(delta.Index, data.ThoughtSignatureDelta)
	case interactions.StepDeltaDataTypeArgumentsDelta:
		a.applyArgumentsDelta(delta.Index, data.ArgumentsDelta)
	default:
		slog.Debug("ignoring unhandled Interactions step delta", "type", data.Type, "index", delta.Index)
	}

	return ""
}

func (a *streamAccumulator) applyTextDelta(index int, delta *interactions.TextDelta) string {
	if delta == nil {
		return ""
	}

	step := a.stepAt(index, interactions.StepTypeModelOutput)
	if step == nil {
		return ""
	}

	appendModelText(step.ModelOutputStep, delta.Text)

	return delta.Text
}

func (a *streamAccumulator) applyThoughtSummary(index int, delta *interactions.ThoughtSummaryDelta) {
	if delta == nil || delta.Content == nil || delta.Content.Type != interactions.ContentTypeText {
		return
	}

	if step := a.stepAt(index, interactions.StepTypeThought); step != nil {
		appendThoughtSummary(step.ThoughtStep, delta.Content.TextContent.Text)
	}
}

func (a *streamAccumulator) applyThoughtSignature(index int, delta *interactions.ThoughtSignatureDelta) {
	if delta == nil || delta.Signature == nil {
		return
	}

	if step := a.stepAt(index, interactions.StepTypeThought); step != nil {
		step.ThoughtStep.Signature = delta.Signature
	}
}

func (a *streamAccumulator) applyArgumentsDelta(index int, delta *interactions.ArgumentsDelta) {
	if delta == nil {
		return
	}

	a.appendArgs(index, utils.Deref(delta.Arguments))
}

func (a *streamAccumulator) appendArgs(index int, partial string) {
	buf, ok := a.args[index]
	if !ok {
		buf = &strings.Builder{}
		a.args[index] = buf
	}

	buf.WriteString(partial)
}

// stopStep finalizes the step at the given index, decoding any streamed function call arguments.
func (a *streamAccumulator) stopStep(stop *interactions.StepStop) error {
	if stop == nil {
		return nil
	}

	a.setUsage(stop.Usage)

	buf, ok := a.args[stop.Index]
	if !ok || buf.Len() == 0 {
		return nil
	}

	step := a.current[stop.Index]
	if step == nil || step.Type != interactions.StepTypeFunctionCall {
		return fmt.Errorf("%w: received function call arguments for non-function step at index %d",
			llms.ErrCompletion, stop.Index)
	}

	raw := buf.String()

	args, err := utils.DecodeJSONObject([]byte(raw))
	if err != nil {
		slog.Warn("failed to decode streamed function call arguments, preserving raw JSON",
			"tool_name", step.FunctionCallStep.Name, "call_id", step.FunctionCallStep.ID, "error", err)

		a.rawArgs[step.FunctionCallStep.ID] = raw

		return nil
	}

	step.FunctionCallStep.Arguments = args

	return nil
}

// stepAt returns the step at index, creating an empty step of the given type if the stream skipped step.start. It
// returns nil if a step of a different type already occupies the index.
func (a *streamAccumulator) stepAt(index int, typ interactions.StepType) *interactions.Step {
	if step, ok := a.current[index]; ok {
		if step.Type != typ {
			slog.Warn("dropping stream delta for mismatched step type", "index", index, "want", typ, "got", step.Type)
			return nil
		}

		return step
	}

	var step interactions.Step

	switch typ { //nolint:exhaustive // Only steps that receive deltas are created here.
	case interactions.StepTypeModelOutput:
		step = interactions.NewStep(interactions.ModelOutputStep{})
	case interactions.StepTypeThought:
		step = interactions.NewStep(interactions.ThoughtStep{})
	default:
		return nil
	}

	a.current[index] = &step

	return &step
}

func appendModelText(output *interactions.ModelOutputStep, text string) {
	if n := len(output.Content); n > 0 && output.Content[n-1].Type == interactions.ContentTypeText {
		output.Content[n-1].TextContent.Text += text
		return
	}

	output.Content = append(output.Content, interactions.NewContent(interactions.TextContent{Text: text}))
}

func appendThoughtSummary(thought *interactions.ThoughtStep, text string) {
	if n := len(thought.Summary); n > 0 && thought.Summary[n-1].Type == interactions.ThoughtSummaryContentTypeText {
		thought.Summary[n-1].TextContent.Text += text
		return
	}

	thought.Summary = append(thought.Summary, interactions.NewThoughtSummaryContent(interactions.TextContent{Text: text}))
}
