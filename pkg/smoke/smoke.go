package smoke

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/cneill/smoke/pkg/llms"
	"github.com/cneill/smoke/pkg/llms/chatgpt"
	"github.com/cneill/smoke/pkg/llms/claude"
	"github.com/cneill/smoke/pkg/prompts"
	"github.com/cneill/smoke/pkg/tools"
)

type Opts struct {
	ProjectPath string

	Debug       bool
	MaxTokens   int64
	Model       string
	SessionName string
	Provider    llms.LLMType
	APIKey      string
}

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
		Name: opts.SessionName,
		// SystemMessage: prompts.System,
		SystemMessage: prompts.SystemJSON(),
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
		chatGPT, err := chatgpt.NewChatGPT(&chatgpt.Opts{
			APIKey: s.opts.APIKey,
			// Model:        openai.ChatModelGPT4o,
			// Model:        openai.ChatModelGPT4_1,
			// Model:        openai.ChatModelO3Mini,
			// Model:        openai.ChatModelGPT5,
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
			APIKey: s.opts.APIKey,
			// Model:        anthropic.ModelClaude4Sonnet20250514,
			// Model:        anthropic.ModelClaudeOpus4_1_20250805,
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

func (s *Smoke) SendUserMessage(ctx context.Context, msg *llms.Message) (*llms.Message, error) {
	s.session.AddMessage(msg)

	response, err := s.llm.SendSession(ctx, s.session)
	if err != nil {
		return nil, fmt.Errorf("failed to send session with user message: %w", err)
	}

	s.session.AddMessage(response)

	return response, nil
}

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

func (s *Smoke) GetMessages() []*llms.Message {
	return s.session.Messages
}

func (s *Smoke) SaveSession(path string) error {
	if path == "" {
		path = fmt.Sprintf("%s_saved_%s.json", s.session.Name, time.Now().Format(time.DateTime))
	}

	sessionBytes, err := json.MarshalIndent(s.session, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal session JSON: %w", err)
	}

	slog.Debug("saving session to file", "path", path)

	if err := os.WriteFile(path, sessionBytes, 0o644); err != nil {
		return fmt.Errorf("failed to write session to file %q: %w", path, err)
	}

	return nil
}
