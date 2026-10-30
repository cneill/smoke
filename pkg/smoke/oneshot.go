package smoke

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/cneill/smoke/pkg/llms"
)

// OneShotRequest describes a single non-interactive prompt to run to completion.
type OneShotRequest struct {
	Prompt string
	// MaxTurns is the maximum number of tool-call rounds allowed before giving up.
	MaxTurns int
	// OnToolCall, if set, is called before each requested tool call is executed.
	OnToolCall func(llms.ToolCall)
}

func (r OneShotRequest) OK() error {
	switch {
	case strings.TrimSpace(r.Prompt) == "":
		return errors.New("prompt must not be empty")
	case r.MaxTurns < 1:
		return fmt.Errorf("max turns must be >0, got %d", r.MaxTurns)
	}

	return nil
}

// RunOneShot sends the prompt to the LLM, executes any requested tool calls, and blocks until the LLM produces its
// final message, which is returned.
func (s *Smoke) RunOneShot(ctx context.Context, req OneShotRequest) (*llms.Message, error) {
	if err := req.OK(); err != nil {
		return nil, fmt.Errorf("invalid one-shot request: %w", err)
	}

	session := s.getMainSession()
	if session == nil {
		return nil, ErrNoSession
	}

	if err := session.AddMessage(llms.SimpleMessage(llms.RoleUser, req.Prompt)); err != nil {
		return nil, fmt.Errorf("failed to add user message to session: %w", err)
	}

	conversation := s.llm.StartConversation(ctx, session)
	defer conversation.Close()

	return oneShotLoop(ctx, session, conversation, req)
}

func oneShotLoop( //nolint:cyclop
	ctx context.Context, session *llms.Session, conversation llms.Conversation, req OneShotRequest,
) (*llms.Message, error) {
	var (
		final  *llms.Message
		turns  int
		events = conversation.Events()
	)

	for {
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("one-shot request canceled: %w", context.Cause(ctx))
		case event, ok := <-events:
			if !ok {
				return finalOrError(final)
			}

			switch event := event.(type) {
			case llms.EventDone:
				return finalOrError(final)
			case llms.EventError:
				return nil, fmt.Errorf("conversation error: %w", event.Err)
			case llms.EventFinalMessage:
				if err := session.AddMessage(event.Message); err != nil {
					return nil, fmt.Errorf("failed to add final message to session: %w", err)
				}

				final = event.Message
			case llms.EventToolCallsRequested:
				turns++
				if turns > req.MaxTurns {
					conversation.Cancel(ErrMaxTurns)
					return nil, fmt.Errorf("%w (%d)", ErrMaxTurns, req.MaxTurns)
				}

				if req.OnToolCall != nil {
					for _, toolCall := range event.Message.ToolCalls() {
						req.OnToolCall(toolCall)
					}
				}

				if err := runToolCalls(ctx, session, conversation, event.Message); err != nil {
					conversation.Cancel(err)
					return nil, err
				}
			case llms.EventUsageUpdate:
				session.UpdateUsage(event.InputTokens, event.OutputTokens)
			case llms.EventTextDelta, llms.EventToolCallResults:
				// Only the final message is printed; these are drained so the conversation keeps moving.
			}
		}
	}
}

func finalOrError(final *llms.Message) (*llms.Message, error) {
	if final == nil || strings.TrimSpace(final.TextContent()) == "" {
		return nil, errors.New("conversation ended without a final answer")
	}

	return final, nil
}
