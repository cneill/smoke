// Package smoke orchestrates the overall interactions with LLMs, session management, tool execution, prompt command
// execution, etc. It is included in the [ui.Model] struct to allow the UI to interact with the state.
package smoke

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/cneill/smoke/internal/uimsg"
	"github.com/cneill/smoke/pkg/commands"
	"github.com/cneill/smoke/pkg/commands/handlers/summarize"
	"github.com/cneill/smoke/pkg/config"
	"github.com/cneill/smoke/pkg/elicit"
	"github.com/cneill/smoke/pkg/llms"
	"github.com/cneill/smoke/pkg/mcp"
	"github.com/cneill/smoke/pkg/plan"
	"github.com/cneill/smoke/pkg/prompts"
	"github.com/cneill/smoke/pkg/skills"
	"github.com/cneill/smoke/pkg/tools"
	"github.com/cneill/smoke/pkg/tools/handlers"
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

	skillCatalog skills.Catalog

	mainSessionName  string
	mainSystemPrompt string
	sessions         map[string]*llms.Session
	sessionMutex     sync.RWMutex

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
	case s.mainSystemPrompt == "":
		return fmt.Errorf("no main system prompt set")
	case s.llmConfig == nil:
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

	if err := smoke.setup(); err != nil {
		return nil, fmt.Errorf("smoke setup failed: %w", err)
	}

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

	if err := smoke.setup(); err != nil {
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

func (s *Smoke) SubmitElicitResponse(response *elicit.Response) error {
	if s.elicitManager == nil {
		return fmt.Errorf("elicit manager not available")
	}

	_, ok := s.elicitManager.ActiveRequest()
	if !ok {
		return fmt.Errorf("no active elicit request")
	}

	if response == nil {
		return fmt.Errorf("missing elicit response")
	}

	if err := s.elicitManager.Complete(response); err != nil {
		return fmt.Errorf("failed to complete elicit request: %w", err)
	}

	return nil
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

func (s *Smoke) HandleSummarizeMessage(msg summarize.SessionSummarizeMessage) (tea.Cmd, error) {
	mainSession := s.getMainSession()
	sessionName := mainSession.Name + "_summary"
	systemMessage := prompts.SummarizeSystemPrompt(msg.OriginalMessages...).Markdown()

	managerOpts := &tools.ManagerOpts{
		ProjectPath:      s.projectPath,
		SessionName:      sessionName,
		ToolInitializers: handlers.SummarizeTools(),
		PlanManager:      s.planManager,
	}

	toolManager, err := tools.NewManager(managerOpts)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize tools manager for summarization conversation: %w", err)
	}

	toolManager.SetTeaEmitter(s.teaEmitter)

	newSession, err := llms.NewSession(&llms.SessionOpts{
		Name:            sessionName,
		SystemMessage:   systemMessage,
		SystemAsMessage: mainSession.SystemAsMessage, // TODO: check LLM for this? something else?
		Tools:           toolManager,
		Mode:            llms.ModeSummarize,
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
			slog.Debug("Starting conversation event-listening loop")
			s.summarizationLoop(context.Background(), msg, newSession, conversation)
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

func (s *Smoke) SetMode(mode llms.Mode) error {
	session := s.getMainSession()
	if session == nil {
		return ErrNoSession
	}

	session.SetMode(mode)

	var (
		enabledTools  []tools.Initializer
		systemMessage string
	)

	switch mode {
	case llms.ModePlanning:
		enabledTools = handlers.PlanningTools()
		systemMessage = prompts.PlanningSystemPrompt().Markdown()
	case llms.ModeReview:
		enabledTools = handlers.ReviewTools()
		systemMessage = prompts.ReviewSystemPrompt().Markdown()
	case llms.ModeWork:
		enabledTools = handlers.WorkTools()
		systemMessage = prompts.WorkSystemPrompt().Markdown()
	default:
		return fmt.Errorf("tried to set smoke to unknown mode %q", mode)
	}

	if err := session.SetSystemMessage(systemMessage); err != nil {
		return fmt.Errorf("failed to set system message for review mode: %w", err)
	}

	session.Tools.InitTools(enabledTools...)

	mcpTools, err := s.getMCPTools()
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
		newMode llms.Mode
	)

	switch oldMode {
	case llms.ModePlanning, llms.ModeRanking, llms.ModeReview, llms.ModeSummarize:
		newMode = llms.ModeWork
	case llms.ModeWork:
		newMode = llms.ModePlanning
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

	session := s.getMainSession()

	for _, mcpClient := range s.mcpClients {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
		defer cancel()

		var (
			mcpTools tools.Tools
			err      error
		)

		switch session.GetMode() {
		case llms.ModePlanning, llms.ModeReview:
			mcpTools, err = mcpClient.PlanTools(ctx)
		case llms.ModeWork:
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
