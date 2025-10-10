// Package smoke orchestrates the overall interactions with LLMs, session management, tool execution, prompt command
// execution, etc. It is included in the [ui.Model] struct to allow the UI to interact with the state.
package smoke

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/cneill/smoke/pkg/commands"
	"github.com/cneill/smoke/pkg/config"
	"github.com/cneill/smoke/pkg/llms"
	"github.com/cneill/smoke/pkg/mcp"
	"github.com/cneill/smoke/pkg/tools"
)

type TeaEmitter func(tea.Msg)

// Smoke manages the overall state of the application, including the project path we're working in, the [*llms.Session]
// we're currently interacting with, the [*tools.Manager] which provides the LLM tool calling affordances, the
// [*commands.Manager] that handles prompt commands from the user, and the actual [llms.LLM] that we're interacting
// with.
type Smoke struct {
	config      *config.Config
	debug       bool
	mode        Mode
	projectPath string

	mainSessionName string
	sessions        map[string]*llms.Session
	sessionMutex    sync.RWMutex

	conversations     map[string]llms.Conversation
	conversationMutex sync.RWMutex

	teaEmitter TeaEmitter

	commands   *commands.Manager
	llmConfig  *llms.Config
	llm        llms.LLM
	mcpClients []*mcp.CommandClient
}

type Mode string

const (
	ModeNormal   = "normal"
	ModePlanning = "planning"
	ModeReview   = "review"
)

func (s *Smoke) OK() error {
	switch {
	case s.projectPath == "":
		return fmt.Errorf("no project path set")
	case s.getMainSession() == nil:
		return fmt.Errorf("no session info set")
	case s.llmConfig == nil || s.llm == nil:
		return fmt.Errorf("no LLM config set")
	}

	return nil
}

func New(opts ...OptFunc) (*Smoke, error) {
	smoke := &Smoke{
		sessions:      map[string]*llms.Session{},
		conversations: map[string]llms.Conversation{},
	}

	var optErr error
	for i, opt := range opts {
		smoke, optErr = opt(smoke)
		if optErr != nil {
			return nil, fmt.Errorf("%w (%d): %w", ErrOptions, i, optErr)
		}
	}

	if err := smoke.OK(); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrOptions, err)
	}

	// Once we've set up the session / etc, add MCP tools as well, if any
	mcpTools, err := smoke.getMCPTools()
	if err != nil {
		return nil, fmt.Errorf("failed to list MCP tools: %w", err)
	}

	smoke.sessionMutex.RLock()
	session := smoke.sessions[smoke.mainSessionName]
	smoke.sessionMutex.RUnlock()

	session.Tools.AddTools(mcpTools...)

	// smoke.session.Tools.AddTools(mcpTools...)

	return smoke, nil
}

func (s *Smoke) Update(opts ...OptFunc) (*Smoke, error) {
	var (
		smoke  *Smoke
		optErr error
	)

	for i, opt := range opts {
		smoke, optErr = opt(s)
		if optErr != nil {
			return nil, fmt.Errorf("%w (%d): %w", ErrOptions, i, optErr)
		}
	}

	if err := smoke.OK(); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrOptions, err)
	}

	// TODO: if opts start cloning Smoke with each iteration, will need to update 's'

	return smoke, nil
}

func (s *Smoke) HandleUserMessage(msg *llms.Message) (tea.Cmd, error) {
	// TODO: for now, we just assume MAIN source and route to the session with the name defined by the user; in the
	// future, this may have to change.
	session := s.getMainSession()
	if session == nil {
		return nil, fmt.Errorf("failed to get main session")
	}

	slog.Debug("Handling user message", "message", msg)

	if err := session.AddMessage(msg); err != nil {
		return nil, fmt.Errorf("failed to add user message to main session: %w", err)
	}

	conversation := s.llm.StartConversation(context.Background(), session)
	s.conversationMutex.Lock()
	// TODO: support other conversations
	s.conversations[s.mainSessionName] = conversation
	s.conversationMutex.Unlock()

	handler := func() tea.Msg {
		defer func() {
			slog.Debug("Closing conversation")
			conversation.Close()
		}()

		wg := sync.WaitGroup{}
		wg.Go(func() {
			slog.Debug("Starting conversation event-listening loop")
			s.conversationLoop(context.Background(), session, conversation)
		})

		wg.Wait()

		return nil
	}

	return handler, nil
}

func (s *Smoke) conversationLoop(ctx context.Context, session *llms.Session, conversation llms.Conversation) {
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
					Err: fmt.Errorf("conversation error: %w", event.Err),
				})
				conversation.Cancel(event.Err)

				return
			case llms.EventFinalMessage:
				pendingMessage = nil

				if err := session.AddMessage(event.Message); err != nil {
					slog.Error("failed to add assistant message to session", "error", err)
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
						llms.WithContent(event.Text),
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
				if err := session.AddMessage(event.Message); err != nil {
					slog.Error("failed to add assistant tool call message to session", "error", err)
					conversation.Cancel(err)
				}

				for _, toolCall := range event.Message.ToolCalls {
					var content string

					output, err := session.Tools.CallTool(ctx, toolCall.Name, toolCall.Args)
					if err != nil {
						slog.Error("failed to call tool", "tool_name", toolCall.Name, "error", err)
						toolCallErr := fmt.Errorf("failed to call tool %q: %w", toolCall.Name, err)
						content = toolCallErr.Error()
					} else {
						content = output
					}

					resultsMsg := llms.NewMessage(
						llms.WithRole(llms.RoleTool),
						llms.WithToolCalls(toolCall),
						llms.WithContent(content),
					)

					if err := session.AddMessage(resultsMsg); err != nil {
						slog.Error("failed to add tool call result message to session", "error", err)
						conversation.Cancel(err)
					}

					slog.Debug("Got assistant tool call message", "message", event.Message)
				}

				if err := conversation.Continue(ctx); err != nil {
					slog.Error("errored out while waiting for continue", "error", err)
					return
				}
			case llms.EventUsageUpdate:
				session.UpdateUsage(event.InputTokens, event.OutputTokens)
				s.teaEmitter(UsageUpdateMessage{
					InputTokens:  event.InputTokens,
					OutputTokens: event.OutputTokens,
				})
			}
		}
	}
}

// SendCommandMessage sends a session to the LLM when triggered by a prompt command.
// TODO: better name
// func (s *Smoke) SendCommandMessage(msg commands.SendSessionMessage) tea.Cmd {
// 	send := func() tea.Msg {
// 		// TODO: add a context.Context here - can't use the global userMessageCancel
// 		response, err := s.llm.SendSession(context.TODO(), msg.Session)
// 		if err != nil {
// 			return SendCommandMessageResponseMessage{
// 				OriginalMessage: msg,
// 				Err:             fmt.Errorf("failed to send session with user message: %w", err),
// 			}
// 		}
//
// 		// TODO: NEED TO ALLOW FOR TOOL CALLS!!!!
// 		slog.Debug("GOT RESPONSE FROM COMMAND MESSAGE", "message", response)
//
// 		if err := msg.Session.AddMessage(response); err != nil {
// 			return SendCommandMessageResponseMessage{
// 				OriginalMessage: msg,
// 				Err:             fmt.Errorf("failed to add assistant response from command run: %w", err),
// 			}
// 		}
//
// 		return SendCommandMessageResponseMessage{
// 			OriginalMessage: msg,
// 			Session:         msg.Session,
// 		}
// 	}
//
// 	return send
// }

// func (s *Smoke) HandleCommandMessageResponse(msg SendCommandMessageResponseMessage) tea.Cmd {
// 	// TODO: handle stuff other than summaries; include full details of original command msg/etc
// 	last := msg.Session.LastByRole(llms.RoleAssistant)
// 	slog.Debug("WE GOT A SUMMARY", "summary", last.Content, "original_message", msg.OriginalMessage)
//
// 	if len(msg.OriginalMessage.OriginalMessages) > 0 {
// 		s.session.ReplaceMessages(msg.OriginalMessage.OriginalMessages, []*llms.Message{last})
// 	}
//
// 	return commands.SessionUpdateMessage{
// 		PromptCommand: msg.OriginalMessage.PromptCommand,
// 		Session:       s.session,
// 		ResetHistory:  true,
// 		Message:       "Summarizing messages",
// 	}.Cmd()
// }

// CancelUserMessage can be triggered by the user pressing the escape key while waiting for an assistant response.
func (s *Smoke) CancelUserMessage(err error) {
	s.conversationMutex.Lock()
	defer s.conversationMutex.Unlock()

	if conv, ok := s.conversations[s.mainSessionName]; ok {
		conv.Cancel(err)
	}

	delete(s.conversations, s.mainSessionName)
}

// GetMessages returns the set of all [*llms.Message] attached to the current [*llms.Session]
func (s *Smoke) GetMessages() []*llms.Message {
	// TODO: read-only copies?
	// TODO: reference different sessions?
	return s.getMainSession().Messages
}

// SetSession overwrites the current [*llms.Session].
func (s *Smoke) SetSession(newSession *llms.Session) error {
	// TODO: FIGURE OUT HOW TO PROPERLY WORK WITH TEARDOWN HERE - CURRENTLY FUCKING UP PLAN WRITING WHEN SWITCHING MODES
	// if s.session != nil {
	// 	if err := s.session.Teardown(); err != nil {
	// 		return fmt.Errorf("failed to tear down previous session when replacing: %w", err)
	// 	}
	// }

	// TODO: set different sessions?
	s.sessionMutex.Lock()
	defer s.sessionMutex.Unlock()

	s.sessions[s.mainSessionName] = newSession

	return nil
}

func (s *Smoke) SetMode(mode Mode) {
	s.mode = mode

	var enabledTools []tools.Initializer

	switch mode {
	case ModePlanning, ModeReview:
		enabledTools = tools.PlanningTools()
	case ModeNormal:
		enabledTools = tools.AllTools()
	}

	session := s.getMainSession()

	session.Tools.InitTools(enabledTools...)

	mcpTools, err := s.getMCPTools()
	if err != nil {
		slog.Error("failed to list MCP tools", "error", err)
	}

	session.Tools.AddTools(mcpTools...)
}

// HandleCommand invokes a prompt command provided by the user.
func (s *Smoke) HandleCommand(msg commands.PromptCommandMessage) (tea.Cmd, error) {
	cmd, err := s.commands.HandleCommand(s.getMainSession(), msg)
	if err != nil {
		return nil, fmt.Errorf("failed to execute prompt command %q: %w", msg.Command, err)
	}

	return cmd, nil
}

func (s *Smoke) CommandCompleter() func(string) []string {
	return s.commands.Completer()
}

// TODO: this feels wrong...
func (s *Smoke) GetUsage() (inputTokens, outputTokens int64) { //nolint:nonamedreturns
	return s.getMainSession().Usage()
}

func (s *Smoke) getMainSession() *llms.Session {
	s.sessionMutex.RLock()
	defer s.sessionMutex.RUnlock()

	if session, ok := s.sessions[s.mainSessionName]; ok {
		return session
	}

	return nil
}

func (s *Smoke) getMCPTools() (tools.Tools, error) {
	results := tools.Tools{}

	for _, mcpClient := range s.mcpClients {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
		defer cancel()

		var (
			mcpTools tools.Tools
			err      error
		)

		switch s.mode {
		case ModePlanning, ModeReview:
			mcpTools, err = mcpClient.PlanTools(ctx)
		case ModeNormal:
			mcpTools, err = mcpClient.Tools(ctx)
		}

		if err != nil {
			if !errors.Is(err, context.Canceled) {
				return nil, fmt.Errorf("error retrieving tools from MCP client %q: %w", mcpClient.Name(), err)
			}

			return nil, fmt.Errorf("context cancelled waiting for tools from MCP client %q: %w", mcpClient.Name(), err)
		}

		results = append(results, mcpTools...)
	}

	return results, nil
}
