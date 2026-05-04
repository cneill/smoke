// Package smoke orchestrates the overall interactions with LLMs, session management, tool execution, prompt command
// execution, etc. It is included in the [ui.Model] struct to allow the UI to interact with the state.
package smoke

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/cneill/smoke/internal/uimsg"
	"github.com/cneill/smoke/pkg/commands"
	"github.com/cneill/smoke/pkg/config"
	"github.com/cneill/smoke/pkg/elicit"
	"github.com/cneill/smoke/pkg/llmctx/agentsmd"
	"github.com/cneill/smoke/pkg/llmctx/modes"
	"github.com/cneill/smoke/pkg/llmctx/skills"
	"github.com/cneill/smoke/pkg/llms"
	"github.com/cneill/smoke/pkg/mcp"
	"github.com/cneill/smoke/pkg/plan"
	"github.com/cneill/smoke/pkg/tools"
)

// Smoke manages the overall state of the application, including the project path we're working in, the [*llms.Session]
// we're currently interacting with, the [*tools.Manager] which provides the LLM tool calling affordances, the
// [*commands.Manager] that handles prompt commands from the user, and the actual [llms.LLM] that we're interacting
// with.
type Smoke struct {
	config      *config.Config
	debug       bool
	projectPath string

	planManager *plan.Manager

	skillCatalog    skills.Catalog
	agentsmdCatalog agentsmd.Catalog

	mainSessionName string
	sessions        map[string]*llms.Session
	sessionMutex    sync.RWMutex

	conversations     map[string]llms.Conversation
	conversationMutex sync.RWMutex

	teaEmitter uimsg.TeaEmitter

	commands      *commands.Manager
	llmConfig     *llms.Config
	llm           llms.LLM
	mcpClients    []*mcp.CommandClient
	elicitManager *elicit.Manager
}

func (s *Smoke) OK() error {
	switch {
	case s.projectPath == "":
		return fmt.Errorf("no project path set")
	case s.mainSessionName == "":
		return fmt.Errorf("no main session name set")
	case s.llmConfig == nil:
		return fmt.Errorf("no LLM config set")
	}

	return nil
}

func New(ctx context.Context, opts ...OptFunc) (*Smoke, error) {
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

	if err := smoke.setup(ctx); err != nil {
		return nil, fmt.Errorf("smoke setup failed: %w", err)
	}

	return smoke, nil
}

func (s *Smoke) Update(ctx context.Context, opts ...OptFunc) (*Smoke, error) {
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

	if err := smoke.setup(ctx); err != nil {
		return nil, fmt.Errorf("smoke setup failed: %w", err)
	}

	// TODO: if opts start cloning Smoke with each iteration, will need to update 's'

	return smoke, nil
}

func (s *Smoke) HandleUserMessage(msg *llms.Message) (tea.Cmd, error) {
	// TODO: for now, we just assume MAIN source and route to the session with the name defined by the user; in the
	// future, this may have to change.
	session := s.getMainSession()
	if session == nil {
		return nil, ErrNoSession
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
				if err := session.AddMessage(event.Message); err != nil {
					slog.Error("failed to add assistant tool call message to session", "error", err)
					conversation.Cancel(err)

					return
				}

				for _, toolCall := range event.Message.ToolCalls {
					var (
						textContent  string
						imageContent []byte
						toolCallErr  error
					)

					output, err := session.Tools.CallTool(ctx, toolCall.Name, toolCall.Args)
					switch {
					case err != nil:
						slog.Error("failed to call tool", "tool_name", toolCall.Name, "error", err)
						toolCallErr = fmt.Errorf("failed to call tool %q: %w", toolCall.Name, err)
						textContent = toolCallErr.Error()
					case output.Type() == tools.OutputTypeText:
						textContent = output.Text
					case output.Type() == tools.OutputTypeImage:
						imageBytes, err := os.ReadFile(output.ImagePath)
						if err != nil {
							slog.Error("failed to read image bytes", "path", output.ImagePath, "error", err)
							toolCallErr = fmt.Errorf("failed to call tool %q: %w", toolCall.Name, err)
							textContent = toolCallErr.Error()
						}

						imageContent = imageBytes
					}

					messageOpts := []llms.MessageOpt{
						llms.WithRole(llms.RoleTool),
						llms.WithToolCalls(toolCall),
					}

					if textContent != "" {
						messageOpts = append(messageOpts, llms.WithTextContent(textContent))
					} else if imageContent != nil {
						messageOpts = append(messageOpts, llms.WithImageContent(imageContent))
					}

					resultsMsg := llms.NewMessage(messageOpts...)

					if toolCallErr != nil {
						resultsMsg = resultsMsg.Update(llms.WithError(toolCallErr))
					}

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

// HandleElicitUserInput takes the raw message sent by the user in response to an elicitation request, parses the
// selected option (or N/A) from it, forwards the message back to the waiting elicit Tool via elicitManager, and returns
// UserResponseMessage with the parsed Response back to the UI to be rendered in the history.
func (s *Smoke) HandleElicitUserInput(msg elicit.UserInputMessage) (elicit.UserResponseMessage, error) {
	var responseMsg elicit.UserResponseMessage

	if s.elicitManager == nil {
		return responseMsg, fmt.Errorf("elicit manager not available")
	}

	response, err := s.elicitManager.ParseUserInput(msg)
	if err != nil {
		return responseMsg, fmt.Errorf("failed to handle user elicit response: %w", err)
	}

	if err := s.elicitManager.Complete(response); err != nil {
		return responseMsg, fmt.Errorf("failed to complete elicit request: %w", err)
	}

	return elicit.UserResponseMessage{Response: response}, nil
}

func (s *Smoke) CancelElicit() error {
	if s.elicitManager == nil {
		return fmt.Errorf("elicit manager not available")
	}

	if err := s.elicitManager.Cancel(); err != nil {
		return fmt.Errorf("failed to cancel elicit request: %w", err)
	}

	return nil
}

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

func (s *Smoke) GetMode() modes.Mode {
	session := s.getMainSession()
	if session == nil {
		return modes.DefaultMode()
	}

	return session.GetMode()
}

func (s *Smoke) SetMode(mode modes.Mode) error {
	session := s.getMainSession()
	if session == nil {
		return ErrNoSession
	}

	session.SetMode(mode)

	systemPrompt, err := s.SystemPrompt(mode)
	if err != nil {
		return err
	}

	if err := session.SetSystemMessage(systemPrompt); err != nil {
		return fmt.Errorf("failed to set system message for mode %q: %w", mode, err)
	}

	enabledTools := s.ModeToolInitializers(mode)
	tools := session.Tools.InitTools(enabledTools...)
	session.Tools.SetTools(tools...)

	ctx, cancel := context.WithTimeout(context.TODO(), time.Second*10)
	defer cancel()

	mcpTools, err := s.GetMCPTools(ctx, mode, s.mcpClients...)
	if err != nil {
		slog.Error("failed to list MCP tools", "error", err)
	}

	session.Tools.AddTools(mcpTools...)

	go s.teaEmitter(ModeMessage{
		Mode: mode,
	})

	return nil
}

func (s *Smoke) ShiftMode() error {
	session := s.getMainSession()
	if session == nil {
		return ErrNoSession
	}

	var (
		oldMode = session.GetMode()
		newMode modes.Mode
	)

	switch oldMode {
	case modes.ModeRanking, modes.ModeReview, modes.ModeSummarize:
		newMode = modes.ModeWork
	case modes.ModeWork:
		newMode = modes.ModePlanning
	case modes.ModePlanning:
		newMode = modes.ModeReview
	}

	slog.Debug("shifting mode", "from", oldMode, "to", newMode)

	return s.SetMode(newMode)
}

// HandleCommand invokes a prompt command provided by the user.
func (s *Smoke) HandleCommand(msg commands.PromptMessage) (tea.Cmd, error) {
	cmd, err := s.commands.HandleCommand(s.getMainSession(), msg)
	if err != nil {
		return nil, fmt.Errorf("failed to execute prompt command %q: %w", msg.Command, err)
	}

	return cmd, nil
}

func (s *Smoke) CommandCompleter() func(string) []string {
	return s.commands.Completer()
}

func (s *Smoke) SkillCompleter() func(string) []string {
	return s.skillCatalog.Completer()
}

func (s *Smoke) GetUsage() (inputTokens, outputTokens int64) { //nolint:nonamedreturns
	// TODO: this feels wrong...
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
