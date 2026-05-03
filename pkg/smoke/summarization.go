package smoke

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/cneill/smoke/internal/uimsg"
	"github.com/cneill/smoke/pkg/commands"
	"github.com/cneill/smoke/pkg/commands/handlers/summarize"
	"github.com/cneill/smoke/pkg/llmctx/modes"
	"github.com/cneill/smoke/pkg/llmctx/prompts"
	"github.com/cneill/smoke/pkg/llms"
)

func (s *Smoke) HandleSummarizeMessage(msg summarize.SessionSummarizeMessage) (tea.Cmd, error) {
	mainSession := s.getMainSession()
	sessionName := mainSession.Name + "_summary"
	systemMessage := prompts.SummarizeSystemPrompt(msg.OriginalMessages...).Markdown()

	toolManager, err := s.NewToolManager(modes.ModeSummarize)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize tools manager for summarization conversation: %w", err)
	}

	newSession, err := llms.NewSession(&llms.SessionOpts{
		Name:            sessionName,
		SystemMessage:   systemMessage,
		SystemAsMessage: mainSession.SystemAsMessage, // TODO: check LLM for this? something else?
		Tools:           toolManager,
		Mode:            modes.ModeSummarize,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to initialize new session for summarization: %w", err)
	}

	userMessage := llms.SimpleMessage(llms.RoleUser, "Please proceed to summarizing the provided messages. Place your "+
		"summarization in your final response, with no additional commentary.")
	if err := newSession.AddMessage(userMessage); err != nil {
		return nil, fmt.Errorf("failed to add user summarization message to summarization session: %w", err)
	}

	slog.Debug("Handling summarization request", "message", msg)

	conversation := s.llm.StartConversation(context.Background(), newSession)
	s.conversationMutex.Lock()
	// TODO: support other conversations
	s.conversations[sessionName] = conversation
	s.conversationMutex.Unlock()

	handler := func() tea.Msg {
		defer func() {
			slog.Debug("Closing summarization conversation")
			conversation.Close()
		}()

		wg := sync.WaitGroup{}
		wg.Go(func() {
			slog.Debug("Starting conversation event-listening loop for summarization")
			s.summarizationLoop(context.TODO(), msg, newSession, conversation)
		})

		wg.Wait()

		return nil
	}

	return handler, nil
}

func (s *Smoke) summarizationLoop(ctx context.Context, msg summarize.SessionSummarizeMessage, session *llms.Session, conversation llms.Conversation) {
	eventsChan := conversation.Events()

	// TODO: smoke message type for returning an error tea.Msg to the UI for things that aren't conversation related,
	// instead of slog.Error()? Channel?

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
					Err: uimsg.ToError(fmt.Errorf("summarization conversation error: %w", event.Err)),
				})
				conversation.Cancel(event.Err)

				return
			case llms.EventFinalMessage:
				if err := session.AddMessage(event.Message); err != nil {
					slog.Error("failed to add assistant message to summarization session", "error", err)
					return
				}

				slog.Debug("Got final assistant message in summarization loop", "message", event.Message)

				num := len(msg.OriginalMessages)

				pluralized := "message"
				if num > 1 {
					pluralized = "messages"
				}

				content := fmt.Sprintf("%s\n\nThis message represents a summary of %d %s, updated %s",
					event.Message.TextContent, num, pluralized, time.Now())

				newMessage := llms.NewMessage(
					llms.WithRole(llms.RoleUser),
					llms.WithTextContent(content),
				)

				mainSession := s.getMainSession()
				mainSession.ReplaceMessages(msg.OriginalMessages, []*llms.Message{newMessage})

				slog.Debug("Emitting request to update session with summarization in UI")

				s.teaEmitter(commands.SessionUpdateMessage{
					PromptMessage: msg.PromptMessage,
					Session:       mainSession,
					ResetHistory:  true,
					Message:       "Summarized requested conversation history and updated main session.",
				})
			case llms.EventTextDelta:
			case llms.EventToolCallResults:
			case llms.EventToolCallsRequested:
				// TODO: break this out to a separate function for use by main conversation loop as well?
				if err := session.AddMessage(event.Message); err != nil {
					slog.Error("failed to add assistant tool call message to session", "error", err)
					conversation.Cancel(err)

					return
				}

				for _, toolCall := range event.Message.ToolCalls {
					var (
						content     string
						toolCallErr error
					)

					output, err := session.Tools.CallTool(ctx, toolCall.Name, toolCall.Args)
					if err != nil {
						slog.Error("failed to call tool", "tool_name", toolCall.Name, "error", err)
						toolCallErr = fmt.Errorf("failed to call tool %q: %w", toolCall.Name, err)
						content = toolCallErr.Error()
					} else {
						// TODO: need to check for images? I doubt it?
						content = output.Text
					}

					resultsMsg := llms.NewMessage(
						llms.WithRole(llms.RoleTool),
						llms.WithToolCalls(toolCall),
						llms.WithTextContent(content),
					)

					if toolCallErr != nil {
						resultsMsg = resultsMsg.Update(llms.WithError(toolCallErr))
					}

					if err := session.AddMessage(resultsMsg); err != nil {
						slog.Error("failed to add tool call result message to session", "error", err)
						conversation.Cancel(err)

						return
					}

					slog.Debug("Got assistant tool call message", "message", event.Message)
				}

				if err := conversation.Continue(ctx); err != nil {
					slog.Error("errored out while waiting for continue", "error", err)
					return
				}
			case llms.EventUsageUpdate:
				// TODO: update main session usage?
			}
		}
	}
}
