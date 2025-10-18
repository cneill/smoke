package smoke

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/cneill/smoke/internal/uimsg"
	"github.com/cneill/smoke/pkg/commands/handlers/rank"
	"github.com/cneill/smoke/pkg/llms"
	"github.com/cneill/smoke/pkg/prompts"
	"github.com/cneill/smoke/pkg/tools"
	"github.com/cneill/smoke/pkg/tools/handlers"
)

func (s *Smoke) HandleRankRequestMessage(msg rank.RequestMessage) (tea.Cmd, error) {
	batchSessions, err := s.batchSessions(msg)
	if err != nil {
		return nil, fmt.Errorf("failed to create individual sessions for batch ranking: %w", err)
	}

	slog.Debug("Handling ranking request", "message", msg)

	commands := []tea.Cmd{}

	for batchIdx, batchSession := range batchSessions {
		conversation := s.llm.StartConversation(context.Background(), batchSession)
		s.conversationMutex.Lock()
		s.conversations[batchSession.Name] = conversation
		s.conversationMutex.Unlock()

		handler := func() tea.Msg {
			defer func() {
				slog.Debug("Closing ranking batch conversation", "idx", batchIdx)
				conversation.Close()
			}()

			wg := sync.WaitGroup{}
			wg.Go(func() {
				slog.Debug("Starting ranking batch conversation event-listening loop", "idx", batchIdx)
				s.handleRankingBatch(context.Background(), msg, batchSession, conversation)
			})

			wg.Wait()

			return nil
		}

		commands = append(commands, handler)
	}

	return tea.Batch(commands...), nil
}

func (s *Smoke) batchSessions(msg rank.RequestMessage) ([]*llms.Session, error) {
	mainSession := s.getMainSession()
	sessions := []*llms.Session{}

	// TODO: figure out the right batch size here
	batches, err := msg.Items.Batch(10)
	if err != nil {
		return nil, fmt.Errorf("failed to create batches from rank request message: %w", err)
	}

	for batchIdx, batch := range batches {
		sessionName := fmt.Sprintf("%s_rank_%d", mainSession.Name, batchIdx)
		systemMessage := prompts.RankSystemPrompt(msg.Description, batch...).Markdown()

		// TODO: For now, this is pretty much irrelevant - there are no ranking tools. Figure out how to rationalize.
		managerOpts := &tools.ManagerOpts{
			ProjectPath:      s.projectPath,
			SessionName:      sessionName,
			ToolInitializers: handlers.RankingTools(),
			PlanManager:      s.planManager,
		}

		toolManager, err := tools.NewManager(managerOpts)
		if err != nil {
			return nil, fmt.Errorf("failed to initialize tools manager for ranking conversation, batch %d: %w", batchIdx, err)
		}

		toolManager.SetTeaEmitter(s.teaEmitter)

		newSession, err := llms.NewSession(&llms.SessionOpts{
			Name:            sessionName,
			SystemMessage:   systemMessage,
			SystemAsMessage: mainSession.SystemAsMessage, // TODO: check LLM for this? something else?
			Tools:           toolManager,
			Mode:            llms.ModeRanking,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to initialize new session for summarization: %w", err)
		}

		userMessage := llms.SimpleMessage(llms.RoleUser, "Please proceed to ranking the provided items according to the instructions.")
		if err := newSession.AddMessage(userMessage); err != nil {
			return nil, fmt.Errorf("failed to add user message to ranking session: %w", err)
		}

		sessions = append(sessions, newSession)
	}

	return sessions, nil
}

func (s *Smoke) handleRankingBatch(ctx context.Context, msg rank.RequestMessage, session *llms.Session, conversation llms.Conversation) {
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
					Err: uimsg.ToError(fmt.Errorf("ranking batch conversation error: %w", event.Err)),
				})
				conversation.Cancel(event.Err)

				return
			case llms.EventFinalMessage:
				if err := session.AddMessage(event.Message); err != nil {
					slog.Error("failed to add assistant message to ranking batch session", "error", err)
					return
				}

				slog.Debug("Got final assistant message in ranking batch loop", "message", event.Message)

			case llms.EventTextDelta:
			case llms.EventToolCallResults:
			case llms.EventToolCallsRequested:
				// TODO: handle this if we ever get ranking tools
			case llms.EventUsageUpdate:
				// TODO: update main session usage?
			}
		}
	}
}
