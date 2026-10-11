package smoke

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/cneill/smoke/internal/uimsg"
	"github.com/cneill/smoke/pkg/llms"
)

func (s *Smoke) conversationLoop(ctx context.Context, session *llms.Session, conversation llms.Conversation) { //nolint:cyclop
	eventsChan := conversation.Events()

	// TODO: smoke message type for returning an error tea.Msg to the UI for things that aren't conversation related,
	// instead of slog.Error()? Channel?

	var pendingMessage *llms.Message

	for {
		select {
		case <-ctx.Done():
			return
		case event, ok := <-eventsChan:
			if !ok {
				return
			}

			switch event := event.(type) {
			case llms.EventDone:
				return
			case llms.EventError:
				slog.Error("conversation error", "error", event.Err)
				s.teaEmitter(AssistantResponseMessage{
					Err: uimsg.ToError(fmt.Errorf("conversation error: %w", event.Err)),
				})
				conversation.Cancel(event.Err)

				return
			case llms.EventFinalMessage:
				pendingMessage = nil

				if err := session.AddMessage(event.Message); err != nil {
					slog.Error("failed to add assistant message to session", "error", err)
					return
				}

				slog.Debug("Got final assistant message", "message", event.Message)

				s.teaEmitter(AssistantResponseMessage{
					Message: event.Message,
				})
			case llms.EventTextDelta:
				// TODO: debounce?
				// TODO: this seems slightly gross to do here...
				if pendingMessage == nil {
					pendingMessage = llms.NewMessage(
						llms.WithRole(llms.RoleAssistant),
						llms.WithID(event.ID),
						llms.WithLLMInfo(s.llm.LLMInfo()),
						llms.WithTextContent(event.Text),
					)
				} else {
					pendingMessage = pendingMessage.Update(
						llms.WithChunkContent(event.Text),
					)
				}

				s.teaEmitter(AssistantUpdatedStreamMessage{
					Message: pendingMessage,
				})
			case llms.EventToolCallResults:
				s.teaEmitter(ToolCallResponseMessage{
					Messages: event.Messages,
				})
			case llms.EventToolCallsRequested:
				pendingMessage = nil

				if err := runToolCalls(ctx, session, conversation, event.Message); err != nil {
					slog.Error("failed to run tool calls", "error", err)
					conversation.Cancel(err)

					return
				}
			case llms.EventUsageUpdate:
				session.UpdateUsage(event.InputTokens, event.OutputTokens)
				s.teaEmitter(UsageUpdateMessage{
					ContextWindowTokens: session.GetUsage().CurrentContextWindowTokens,
				})
			}
		}
	}
}

// runToolCalls records the assistant's tool call request in the session, executes each requested tool call, records
// the results, and then signals the conversation to continue.
func runToolCalls(ctx context.Context, session *llms.Session, conversation llms.Conversation, msg *llms.Message) error {
	if err := session.AddMessage(msg); err != nil {
		return fmt.Errorf("failed to add assistant tool call message to session: %w", err)
	}

	slog.Debug("Got assistant tool call message", "message", msg)

	for _, toolCall := range msg.ToolCalls() {
		resultsMsg := toolCallResultMessage(ctx, session, toolCall)

		if err := session.AddMessage(resultsMsg); err != nil {
			return fmt.Errorf("failed to add tool call result message to session: %w", err)
		}
	}

	if err := conversation.Continue(ctx); err != nil {
		return fmt.Errorf("failed to continue conversation after tool calls: %w", err)
	}

	return nil
}
