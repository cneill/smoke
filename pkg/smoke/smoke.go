// Package smoke orchestrates the overall interactions with LLMs, session management, tool execution, prompt command
// execution, etc. It is included in the [ui.Model] struct to allow the UI to interact with the state.
package smoke

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/cneill/smoke/pkg/commands"
	"github.com/cneill/smoke/pkg/config"
	"github.com/cneill/smoke/pkg/llms"
	"github.com/cneill/smoke/pkg/mcp"
	"github.com/cneill/smoke/pkg/tools"
)

// Smoke manages the overall state of the application, including the project path we're working in, the [*llms.Session]
// we're currently interacting with, the [*tools.Manager] which provides the LLM tool calling affordances, the
// [*commands.Manager] that handles prompt commands from the user, and the actual [llms.LLM] that we're interacting
// with.
type Smoke struct {
	config *config.Config
	debug  bool
	mode   Mode

	projectPath       string
	session           *llms.Session
	commands          *commands.Manager
	llmConfig         *llms.Config
	llm               llms.LLM
	userMessageCancel context.CancelCauseFunc
	mcpClients        []*mcp.CommandClient
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
	case s.session == nil:
		return fmt.Errorf("no session info set")
	case s.llmConfig == nil || s.llm == nil:
		return fmt.Errorf("no LLM config set")
	}

	return nil
}

func New(opts ...OptFunc) (*Smoke, error) {
	smoke := &Smoke{}

	var optErr error
	for _, opt := range opts {
		smoke, optErr = opt(smoke)
		if optErr != nil {
			return nil, fmt.Errorf("%w: %w", ErrOptions, optErr)
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

	smoke.session.Tools.AddTools(mcpTools...)

	return smoke, nil
}

// SendUserMessage appends 'msg' to the current [*llms.Session], invokes the [llms.LLM] to send that session to the
// provider, then adds the response to the session as well.
func (s *Smoke) SendUserMessage(msg *llms.Message) (tea.Cmd, error) {
	if err := s.session.AddMessage(msg); err != nil {
		return nil, fmt.Errorf("failed to add user message: %w", err)
	}

	send := func() tea.Msg {
		// TODO: WithTimeout?
		ctx, cancel := context.WithCancelCause(context.Background())

		defer func() {
			cancel(nil)

			s.userMessageCancel = nil
		}()
		// TODO: handle multiple requests in-flight
		s.userMessageCancel = cancel

		response, err := s.llm.SendSession(ctx, s.session)
		if err != nil {
			if errors.Is(err, context.Canceled) {
				return AssistantResponseMessage{Err: fmt.Errorf("%w: %w", err, context.Cause(ctx))}
			}

			return AssistantResponseMessage{Err: fmt.Errorf("failed to send session with user message: %w", err)}
		}

		if err := s.session.AddMessage(response); err != nil {
			return AssistantResponseMessage{Err: fmt.Errorf("failed to add assistant response message: %w", err)}
		}

		return AssistantResponseMessage{Message: response}
	}

	return send, nil
}

func (s *Smoke) SendUserMessageStreaming(msg *llms.Message, chunkChan chan *llms.Message) (tea.Cmd, error) {
	llm, ok := s.llm.(llms.StreamingLLM)
	if !ok {
		return nil, fmt.Errorf("streaming is not supported by this LLM (%s, %s)", s.llmConfig.Provider, s.llmConfig.Model)
	}

	if err := s.session.AddMessage(msg); err != nil {
		return nil, fmt.Errorf("failed to add user message: %w", err)
	}

	send := func() tea.Msg {
		slog.Debug("sending session, starting streaming")

		// TODO: WithTimeout?
		ctx, cancel := context.WithCancelCause(context.Background())
		s.userMessageCancel = cancel

		defer func() {
			cancel(nil)

			s.userMessageCancel = nil
		}()

		response, err := llm.SendSessionStreaming(ctx, s.session, chunkChan)
		if err != nil {
			if errors.Is(err, context.Canceled) {
				return AssistantResponseMessage{Err: fmt.Errorf("%w: %w", err, context.Cause(ctx))}
			}

			return AssistantResponseMessage{Err: fmt.Errorf("failed to send session with user message: %w", err)}
		}

		if err := s.session.AddMessage(response); err != nil {
			return AssistantResponseMessage{Err: fmt.Errorf("failed to add assistant response message: %w", err)}
		}

		return AssistantResponseMessage{Message: response}
	}

	return send, nil
}

// SendCommandMessage sends a session to the LLM when triggered by a prompt command.
// TODO: better name
func (s *Smoke) SendCommandMessage(msg commands.SendSessionMessage) tea.Cmd {
	send := func() tea.Msg {
		// TODO: add a context.Context here - can't use the global userMessageCancel
		response, err := s.llm.SendSession(context.TODO(), msg.Session)
		if err != nil {
			return SendCommandMessageResponseMessage{
				OriginalMessage: msg,
				Err:             fmt.Errorf("failed to send session with user message: %w", err),
			}
		}

		if err := msg.Session.AddMessage(response); err != nil {
			return SendCommandMessageResponseMessage{
				OriginalMessage: msg,
				Err:             fmt.Errorf("failed to add assistant response from command run: %w", err),
			}
		}

		return SendCommandMessageResponseMessage{
			OriginalMessage: msg,
			Session:         msg.Session,
		}
	}

	return send
}

func (s *Smoke) HandleCommandMessageResponse(msg SendCommandMessageResponseMessage) tea.Cmd {
	// TODO: handle stuff other than summaries; include full details of original command msg/etc
	last := msg.Session.LastByRole(llms.RoleAssistant)
	slog.Debug("WE GOT A SUMMARY", "summary", last.Content, "original_message", msg.OriginalMessage)

	if len(msg.OriginalMessage.OriginalMessages) > 0 {
		s.session.ReplaceMessages(msg.OriginalMessage.OriginalMessages, []*llms.Message{last})
	}

	return commands.SessionUpdateMessage{
		PromptCommand: msg.OriginalMessage.PromptCommand,
		Session:       s.session,
		ResetHistory:  true,
		Message:       "Summarizing messages",
	}.Cmd()
}

// CancelUserMessage can be triggered by the user pressing the escape key while waiting for an assistant response.
func (s *Smoke) CancelUserMessage(err error) {
	if s.userMessageCancel != nil {
		s.userMessageCancel(err)
		s.userMessageCancel = nil
	}
}

// HandleAssistantToolCalls invokes the [llms.LLM] to execute any tools called within the last assistant message and
// returns the resulting [*llms.Message] objects.
func (s *Smoke) HandleAssistantToolCalls(msg *llms.Message) (tea.Cmd, error) {
	if !msg.HasToolCalls() {
		return nil, llms.ErrNoToolCalls
	}

	handle := func() tea.Msg {
		results, err := s.llm.HandleToolCalls(context.Background(), msg, s.session)
		if err != nil {
			return ToolCallResponseMessage{Err: fmt.Errorf("error handling tool calls: %w", err)}
		}

		return ToolCallResponseMessage{Messages: results}
	}

	return handle, nil
}

// HandleToolCallResults receives the slice of [*llms.Message] resulting from one or more tool calls by the [llms.LLM],
// adds these to the current [*llms.Session], and sends the session to the provider. It then appends the response to the
// current session.
func (s *Smoke) HandleToolCallResults(messages []*llms.Message) (tea.Cmd, error) {
	for i, message := range messages {
		if err := s.session.AddMessage(message); err != nil {
			return nil, fmt.Errorf("failed to add message for tool call %d: %w", i, err)
		}
	}

	handle := func() tea.Msg {
		ctx, cancel := context.WithCancelCause(context.Background())

		defer func() {
			cancel(nil)

			s.userMessageCancel = nil
		}()

		s.userMessageCancel = cancel

		// TODO: combine SendUserMessage + this in a more coherent way?
		response, err := s.llm.SendSession(ctx, s.session)
		if err != nil {
			if errors.Is(err, context.Canceled) {
				return AssistantResponseMessage{Err: fmt.Errorf("%w: %w", err, context.Cause(ctx))}
			}

			return AssistantResponseMessage{Err: fmt.Errorf("failed to send session with tool call results: %w", err)}
		}

		if err := s.session.AddMessage(response); err != nil {
			return AssistantResponseMessage{Err: fmt.Errorf("failed to add assistant response to tool call: %w", err)}
		}

		return AssistantResponseMessage{Message: response}
	}

	return handle, nil
}

// GetMessages returns the set of all [*llms.Message] attached to the current [*llms.Session]
func (s *Smoke) GetMessages() []*llms.Message {
	return s.session.Messages
}

// SetSession overwrites the current [*llms.Session].
func (s *Smoke) SetSession(newSession *llms.Session) error {
	// TODO: FIGURE OUT HOW TO PROPERLY WORK WITH TEARDOWN HERE - CURRENTLY FUCKING UP PLAN WRITING WHEN SWITCHING MODES
	// if s.session != nil {
	// 	if err := s.session.Teardown(); err != nil {
	// 		return fmt.Errorf("failed to tear down previous session when replacing: %w", err)
	// 	}
	// }

	s.session = newSession

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

	s.session.Tools.InitTools(enabledTools...)

	mcpTools, err := s.getMCPTools()
	if err != nil {
		slog.Error("failed to list MCP tools", "error", err)
	}

	s.session.Tools.AddTools(mcpTools...)
}

// HandleCommand invokes a prompt command provided by the user.
func (s *Smoke) HandleCommand(msg commands.PromptCommandMessage) (tea.Cmd, error) {
	cmd, err := s.commands.HandleCommand(s.session, msg)
	if err != nil {
		return nil, fmt.Errorf("failed to execute command %q: %w", msg.Command, err)
	}

	return cmd, nil
}

func (s *Smoke) CommandCompleter() func(string) []string {
	return s.commands.Completer()
}

// TODO: this feels wrong...
func (s *Smoke) GetUsage() (inputTokens, outputTokens int64) { //nolint:nonamedreturns
	return s.session.Usage()
}

func (s *Smoke) ShouldStream() bool {
	// TODO: additional toggle switch for this behavior from CLI flags / env vars / config file

	// GPT-5 requires org verification with a photo ID
	if s.llm.LLMInfo().Type == llms.LLMTypeChatGPT {
		if strings.Contains(s.llm.LLMInfo().ModelName, "gpt-5") {
			return false
		}
	}

	_, ok := s.llm.(llms.StreamingLLM)

	return ok
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
