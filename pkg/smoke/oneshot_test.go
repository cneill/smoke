package smoke //nolint:testpackage

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"testing"

	"github.com/cneill/smoke/pkg/llms"
	"github.com/cneill/smoke/pkg/tools"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeConversation struct {
	events    chan llms.Event
	continues int
	cancelErr error
	canceled  bool
}

func (f *fakeConversation) ID() string                     { return "fake" }
func (f *fakeConversation) Events() <-chan llms.Event      { return f.events }
func (f *fakeConversation) Start(context.Context)          {}
func (f *fakeConversation) Continue(context.Context) error { f.continues++; return nil }
func (f *fakeConversation) Close()                         {}

func (f *fakeConversation) Cancel(err error) {
	f.canceled = true
	f.cancelErr = err
}

func finalEvent(text string) llms.Event {
	return llms.EventFinalMessage{Message: llms.NewMessage(
		llms.WithRole(llms.RoleAssistant),
		llms.WithTextContent(text),
	)}
}

func toolCallEvent(n int) llms.Event {
	return llms.EventToolCallsRequested{Message: llms.NewMessage(
		llms.WithRole(llms.RoleAssistant),
		llms.WithToolCalls(llms.ToolCall{
			ID:   fmt.Sprintf("call_%d", n),
			Name: "recording_tool",
			Args: tools.Args{"path": "README.md"},
		}),
	)}
}

func TestOneShotLoop(t *testing.T) { //nolint:funlen
	t.Parallel()

	errBoom := errors.New("boom")

	tests := []struct {
		name string
		// events are pre-loaded into a closed channel; if nil, the channel is left open and empty.
		events        []llms.Event
		cancelCtx     bool
		maxTurns      int
		wantText      string
		wantErr       error
		wantAnyErr    bool
		wantToolRuns  int64
		wantContinues int
		wantCanceled  bool
	}{
		{
			name: "final_message",
			events: []llms.Event{
				llms.EventTextDelta{ID: "1", Text: "hel"},
				llms.EventUsageUpdate{InputTokens: 10, OutputTokens: 5},
				finalEvent("hello"),
				llms.EventDone{},
			},
			maxTurns: 5,
			wantText: "hello",
		},
		{
			name: "tool_call_then_final",
			events: []llms.Event{
				toolCallEvent(1),
				llms.EventToolCallResults{},
				finalEvent("done"),
				llms.EventDone{},
			},
			maxTurns:      5,
			wantText:      "done",
			wantToolRuns:  1,
			wantContinues: 1,
		},
		{
			name: "exceeds_max_turns",
			events: []llms.Event{
				toolCallEvent(1),
				toolCallEvent(2),
				finalEvent("never"),
				llms.EventDone{},
			},
			maxTurns:      1,
			wantErr:       ErrMaxTurns,
			wantToolRuns:  1,
			wantContinues: 1,
			wantCanceled:  true,
		},
		{
			name:       "event_error",
			events:     []llms.Event{llms.EventError{Err: errBoom}},
			maxTurns:   5,
			wantErr:    errBoom,
			wantAnyErr: true,
		},
		{
			name:       "done_without_final",
			events:     []llms.Event{llms.EventDone{}},
			maxTurns:   5,
			wantAnyErr: true,
		},
		{
			name:       "context_canceled",
			events:     nil,
			cancelCtx:  true,
			maxTurns:   5,
			wantErr:    context.Canceled,
			wantAnyErr: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			calls := &atomic.Int64{}
			manager, err := tools.NewManager(&tools.ManagerOpts{
				ProjectPath: t.TempDir(),
				SessionName: "test-session",
			})
			require.NoError(t, err)
			manager.SetTools(recordingTool{calls: calls})

			session := testSession(t, manager)
			require.NoError(t, session.AddMessage(llms.SimpleMessage(llms.RoleUser, "question")))

			conv := &fakeConversation{}
			if test.events == nil {
				conv.events = make(chan llms.Event)
			} else {
				conv.events = make(chan llms.Event, len(test.events))
				for _, event := range test.events {
					conv.events <- event
				}

				close(conv.events)
			}

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			if test.cancelCtx {
				cancel()
			}

			var seen []string

			req := OneShotRequest{
				Prompt:     "question",
				MaxTurns:   test.maxTurns,
				OnToolCall: func(tc llms.ToolCall) { seen = append(seen, tc.Name) },
			}

			final, err := oneShotLoop(ctx, session, conv, req)

			if test.wantErr != nil || test.wantAnyErr {
				require.Error(t, err)

				if test.wantErr != nil {
					require.ErrorIs(t, err, test.wantErr)
				}

				assert.Nil(t, final)
			} else {
				require.NoError(t, err)
				require.NotNil(t, final)
				assert.Equal(t, test.wantText, final.TextContent())
			}

			assert.Equal(t, test.wantToolRuns, calls.Load())
			assert.Len(t, seen, int(test.wantToolRuns))
			assert.Equal(t, test.wantContinues, conv.continues)
			assert.Equal(t, test.wantCanceled, conv.canceled)

			if test.wantCanceled {
				assert.ErrorIs(t, conv.cancelErr, ErrMaxTurns)
			}
		})
	}
}
