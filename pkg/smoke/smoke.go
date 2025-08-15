// Package smoke orchestrates the overall interactions with LLMs, session management, tool execution, prompt command
// execution, etc. It is included in the [ui.Model] struct to allow the UI to interact with the state.
package smoke

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/anthropics/anthropic-sdk-go"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/cneill/smoke/pkg/commands"
	"github.com/cneill/smoke/pkg/llms"
	"github.com/cneill/smoke/pkg/llms/chatgpt"
	"github.com/cneill/smoke/pkg/llms/claude"
	"github.com/cneill/smoke/pkg/prompts"
	"github.com/cneill/smoke/pkg/tools"
)

// Opts tells us how to configure the LLM, what project directory we'll work within, etc.
type Opts struct {
	ProjectPath string

	Debug       bool
	MaxTokens   int64
	Model       string
	SessionName string
	Provider    llms.LLMType
	APIKey      string
}

// OK validates that we have valid values for all options.
func (o *Opts) OK() error {
	switch {
	case o.ProjectPath == "":
		return fmt.Errorf("missing project path")
	case o.Model == "":
		return fmt.Errorf("missing model")
	case o.APIKey == "":
		return fmt.Errorf("missing api key")
	}

	return nil
}

// Smoke manages the overall state of the application, including the project path we're working in, the [*llms.Session]
// we're currently interacting with, the [*tools.Manager] which provides the LLM tool calling affordances, the
// [*commands.Manager] that handles prompt commands from the user, and the actual [llms.LLM] that we're interacting
// with.
type Smoke struct {
	opts *Opts

	ProjectPath string
	session     *llms.Session
	tools       *tools.Manager
	commands    *commands.Manager
	llm         llms.LLM
}

// New validates that the provided ProjectPath exists and contains a .git subdirectory, storing the absolute path if the
// location specified is valid. It creates a new [*llms.Session] with the provided name.
func New(opts *Opts) (*Smoke, error) {
	if err := opts.OK(); err != nil {
		return nil, fmt.Errorf("options error: %w", err)
	}

	absPath, err := filepath.Abs(opts.ProjectPath)
	if err != nil {
		return nil, fmt.Errorf("invalid project path %q: %w", absPath, err)
	}

	if _, err := os.Stat(absPath); err != nil {
		return nil, fmt.Errorf("failed to stat project path %q: %w", absPath, err)
	}

	gitPath := filepath.Join(absPath, ".git")
	if _, err := os.Stat(gitPath); err != nil {
		return nil, fmt.Errorf("failed to stat '.git' directory in project path %q: %w", gitPath, err)
	}

	session, err := llms.NewSession(&llms.SessionOpts{
		Name: opts.SessionName,
		// SystemMessage: prompts.System,
		SystemMessage: prompts.SystemJSON(),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to initialize session: %w", err)
	}

	smoke := &Smoke{
		opts: opts,

		session:  session,
		tools:    tools.NewManager(absPath),
		commands: commands.NewManager(absPath),
	}

	if err := smoke.setupLLM(); err != nil {
		return nil, fmt.Errorf("failed to set up LLM: %w", err)
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

// setupLLM provides the configuration details necessary to set up the [llms.LLM] based on the provider. It saves this
// for later use.
func (s *Smoke) setupLLM() error {
	var llm llms.LLM

	switch s.opts.Provider {
	case llms.LLMTypeChatGPT:
		chatGPT, err := chatgpt.NewChatGPT(&chatgpt.Opts{
			APIKey:       s.opts.APIKey,
			Model:        s.opts.Model,
			MaxTokens:    s.opts.MaxTokens,
			ToolsManager: s.tools,
		})
		if err != nil {
			return fmt.Errorf("failed to initialize ChatGPT client: %w", err)
		}

		llm = chatGPT

	case llms.LLMTypeClaude:
		claude, err := claude.NewClaude(&claude.Opts{
			APIKey:       s.opts.APIKey,
			Model:        anthropic.Model(s.opts.Model),
			MaxTokens:    s.opts.MaxTokens,
			ToolsManager: s.tools,
		})
		if err != nil {
			return fmt.Errorf("failed to initialize Claude client: %w", err)
		}

		llm = claude

	default:
		return fmt.Errorf("unknown LLM provider: %s", s.opts.Provider)
	}

	if llm.RequiresSessionSystem() {
		s.session.AddMessage(llms.SimpleMessage(llms.RoleSystem, s.session.SystemMessage))
	}

	s.llm = llm

	return nil
}
