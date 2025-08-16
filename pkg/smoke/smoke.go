// Package smoke orchestrates the overall interactions with LLMs, session management, tool execution, prompt command
// execution, etc. It is included in the [ui.Model] struct to allow the UI to interact with the state.
package smoke

import (
	"context"
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

	projectPath string
	session     *llms.Session
	tools       *tools.Manager
	commands    *commands.Manager
	llmConfig   *llms.Config
	llm         llms.LLM
}

func (s *Smoke) OK() error {
	switch {
	case s.tools == nil || s.commands == nil:
		return fmt.Errorf("no project path provided")
	case s.session == nil:
		return fmt.Errorf("no session info provided")
	case s.llmConfig == nil || s.llm == nil:
		return fmt.Errorf("no LLM config provided")
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
func (s *Smoke) SendUserMessage(ctx context.Context, msg *llms.Message) (*llms.Message, error) {
	s.session.AddMessage(msg)

	response, err := s.llm.SendSession(ctx, s.session)
	if err != nil {
		return nil, fmt.Errorf("failed to send session with user message: %w", err)
	}

	s.session.AddMessage(response)

	return response, nil
}

// HandleAssistantToolCalls involes the [llms.LLM] to execute any tools called within the last assistant message and
// returns the resulting [*llms.Message] objects.
func (s *Smoke) HandleAssistantToolCalls(msg *llms.Message) ([]*llms.Message, error) {
	if !msg.HasToolCalls() {
		return nil, llms.ErrNoToolCalls
	}

	results, err := s.llm.HandleToolCalls(msg)
	if err != nil {
		return nil, fmt.Errorf("error handling tool calls: %w", err)
	}

	return results, nil
}

// HandleToolCallResults receives the slice of [*llms.Message] resulting from one or more tool calls by the [llms.LLM],
// adds these to the current [*llms.Session], and sends the session to the provider. It then appends the response to the
// current session.
func (s *Smoke) HandleToolCallResults(ctx context.Context, messages []*llms.Message) (*llms.Message, error) {
	for _, message := range messages {
		s.session.AddMessage(message)
	}

	// TODO: combine SendUserMessage + this in a more coherent way?
	response, err := s.llm.SendSession(ctx, s.session)
	if err != nil {
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
	// TODO: update system prompt
	// TODO: update LLM tools manager
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
