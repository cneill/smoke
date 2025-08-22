// Package smoke orchestrates the overall interactions with LLMs, session management, tool execution, prompt command
// execution, etc. It is included in the [ui.Model] struct to allow the UI to interact with the state.
package smoke

import (
	"context"
	"errors"
	"fmt"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/cneill/smoke/pkg/commands"
	"github.com/cneill/smoke/pkg/llms"
	"github.com/cneill/smoke/pkg/tools"
)

// Smoke manages the overall state of the application, including the project path we're working in, the [*llms.Session]
// we're currently interacting with, the [*tools.Manager] which provides the LLM tool calling affordances, the
// [*commands.Manager] that handles prompt commands from the user, and the actual [llms.LLM] that we're interacting
// with.
type Smoke struct {
	debug        bool
	planningMode bool

	projectPath       string
	session           *llms.Session
	commands          *commands.Manager
	llmConfig         *llms.Config
	llm               llms.LLM
	userMessageCancel context.CancelCauseFunc
}

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

	return smoke, nil
}

// SendUserMessage appends 'msg' to the current [*llms.Session], invokes the [llms.LLM] to send that session to the
// provider, then adds the response to the session as well.
func (s *Smoke) SendUserMessage(msg *llms.Message) (*llms.Message, error) {
	s.session.AddMessage(msg)

	// TODO: WithTimeout?
	ctx, cancel := context.WithCancelCause(context.Background())

	defer func() {
		cancel(fmt.Errorf("request complete"))

		s.userMessageCancel = nil
	}()
	// TODO: handle multiple requests in-flight
	s.userMessageCancel = cancel

	response, err := s.llm.SendSession(ctx, s.session)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			return nil, fmt.Errorf("%w: %w", err, context.Cause(ctx))
		}

		return nil, fmt.Errorf("failed to send session with user message: %w", err)
	}

	s.session.AddMessage(response)

	return response, nil
}

// CancelUserMessage can be triggered by the user pressing the escape key while waiting for an assistant response.
func (s *Smoke) CancelUserMessage(err error) {
	if s.userMessageCancel != nil {
		s.userMessageCancel(err)
		s.userMessageCancel = nil
	}
}

// HandleAssistantToolCalls involes the [llms.LLM] to execute any tools called within the last assistant message and
// returns the resulting [*llms.Message] objects.
func (s *Smoke) HandleAssistantToolCalls(msg *llms.Message) ([]*llms.Message, error) {
	if !msg.HasToolCalls() {
		return nil, llms.ErrNoToolCalls
	}

	// TODO: accept session in the function params instead of using the one bolted onto the struct?
	results, err := s.llm.HandleToolCalls(msg, s.session)
	if err != nil {
		return nil, fmt.Errorf("error handling tool calls: %w", err)
	}

	return results, nil
}

// HandleToolCallResults receives the slice of [*llms.Message] resulting from one or more tool calls by the [llms.LLM],
// adds these to the current [*llms.Session], and sends the session to the provider. It then appends the response to the
// current session.
func (s *Smoke) HandleToolCallResults(messages []*llms.Message) (*llms.Message, error) {
	for _, message := range messages {
		s.session.AddMessage(message)
	}

	ctx, cancel := context.WithCancelCause(context.Background())

	defer func() {
		cancel(fmt.Errorf("tool call result request complete"))

		s.userMessageCancel = nil
	}()

	s.userMessageCancel = cancel

	// TODO: combine SendUserMessage + this in a more coherent way?
	response, err := s.llm.SendSession(ctx, s.session)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			return nil, fmt.Errorf("%w: %w", err, context.Cause(ctx))
		}

		return nil, fmt.Errorf("failed to send session with tool call results: %w", err)
	}

	s.session.AddMessage(response)

	return response, nil
}

// GetMessages returns the set of all [*llms.Message] attached to the current [*llms.Session]
func (s *Smoke) GetMessages() []*llms.Message {
	return s.session.Messages
}

// SetSession overwrites the current [*llms.Session].
func (s *Smoke) SetSession(newSession *llms.Session) {
	s.session = newSession
}

// SetPlanningMode enables or disables planning mode.
func (s *Smoke) SetPlanningMode(enabled bool) {
	s.planningMode = enabled
	if enabled {
		s.session.Tools.SetTools(tools.PlanningTools()...)
	} else {
		s.session.Tools.SetTools(tools.AllTools()...)
	}

	// TODO: update system prompt?
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
