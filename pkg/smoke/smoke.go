package smoke

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/cneill/smoke/pkg/llms"
	"github.com/cneill/smoke/pkg/prompts"
	"github.com/cneill/smoke/pkg/tools"
	"github.com/openai/openai-go/v2"
)

type Opts struct {
	ProjectPath string

	Debug       bool
	MaxTokens   int64
	SessionName string
	Provider    llms.LLMType
	APIKey      string
}

func (o *Opts) OK() error {
	switch {
	case o.ProjectPath == "":
		return fmt.Errorf("missing project path")
	case o.APIKey == "":
		return fmt.Errorf("missing api key")
	}

	return nil
}

type Smoke struct {
	opts *Opts

	ProjectPath string
	session     *llms.Session
	tools       *tools.Manager
	llm         llms.LLM
}

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
		Name:          opts.SessionName,
		SystemMessage: prompts.System,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to initialize session: %w", err)
	}

	smoke := &Smoke{
		opts: opts,

		session: session,
		tools:   tools.NewManager(absPath),
	}

	if err := smoke.setupLLM(); err != nil {
		return nil, fmt.Errorf("failed to set up LLM: %w", err)
	}

	return smoke, nil
}

func (s *Smoke) setupLLM() error {
	var llm llms.LLM

	switch s.opts.Provider {
	case llms.LLMTypeChatGPT:
		chatGPT, err := llms.NewChatGPT(&llms.ChatGPTOpts{
			APIKey: s.opts.APIKey,
			// Model:        openai.ChatModelGPT4o,
			// Model:        openai.ChatModelGPT4_1,
			// Model:        openai.ChatModelO3Mini,
			Model:        openai.ChatModelGPT5,
			MaxTokens:    s.opts.MaxTokens,
			ToolsManager: s.tools,
		})
		if err != nil {
			return fmt.Errorf("failed to initialize ChatGPT client: %w", err)
		}

		llm = chatGPT

	case llms.LLMTypeClaude:
		claude, err := llms.NewClaude(&llms.ClaudeOpts{
			APIKey:       s.opts.APIKey,
			Model:        anthropic.ModelClaude4Sonnet20250514,
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

	s.llm = llm

	return nil
}

func (s *Smoke) SendUserMessage(ctx context.Context, msg *llms.Message) (*llms.Message, error) {
	s.session.AddMessage(msg)

	if err := s.llm.SendSession(ctx, s.session); err != nil {
		return nil, fmt.Errorf("failed to send session: %w", err)
	}

	last := s.session.LastByRole(llms.RoleAssistant)

	return last, nil
}

func (s *Smoke) GetMessages() []*llms.Message {
	return s.session.Messages
}
