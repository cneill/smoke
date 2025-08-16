package smoke

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/cneill/smoke/pkg/commands"
	"github.com/cneill/smoke/pkg/llms"
	"github.com/cneill/smoke/pkg/llms/chatgpt"
	"github.com/cneill/smoke/pkg/llms/claude"
	"github.com/cneill/smoke/pkg/tools"
)

// OptFunc is used to configure aspects of Smoke.
type OptFunc func(smoke *Smoke) (*Smoke, error)

// WithSessionInfo configures the details of the session we'll work with.
func WithSessionInfo(name, systemPrompt string) OptFunc {
	return func(smoke *Smoke) (*Smoke, error) {
		session, err := llms.NewSession(&llms.SessionOpts{
			Name:          name,
			SystemMessage: systemPrompt,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to initialize session: %w", err)
		}

		smoke.session = session

		return smoke, nil
	}
}

// WithProjectPath sets the directory we'll work from, and configures the tools and commands managers.
func WithProjectPath(path string) OptFunc {
	return func(smoke *Smoke) (*Smoke, error) {
		if smoke.session == nil {
			return nil, fmt.Errorf("must set session info before project path")
		}

		absPath, err := filepath.Abs(path)
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

		smoke.projectPath = absPath
		smoke.tools = tools.NewManager(absPath, smoke.session.Name)
		smoke.commands = commands.NewManager(absPath)

		return smoke, nil
	}
}

// WithDebug sets the logging level to debug.
func WithDebug(value bool) OptFunc {
	return func(smoke *Smoke) (*Smoke, error) {
		smoke.debug = value
		return smoke, nil
	}
}

// WithLLMConfig validates the LLM config and sets up the [llms.LLM] we'll work with. This option must come after
// WithProjectPath and WithSessionInfo.
func WithLLMConfig(config *llms.Config) OptFunc {
	return func(smoke *Smoke) (*Smoke, error) {
		if err := config.OK(); err != nil {
			return nil, fmt.Errorf("LLM config: %w", err)
		}

		if smoke.tools == nil {
			return nil, fmt.Errorf("must set project path before LLM config")
		}

		if smoke.session == nil {
			return nil, fmt.Errorf("must set session info before LLM config")
		}

		smoke.llmConfig = config

		var (
			llm llms.LLM
			err error
		)

		switch config.Provider {
		case llms.LLMTypeChatGPT:
			llm, err = chatgpt.New(config, smoke.tools)
		case llms.LLMTypeClaude:
			llm, err = claude.New(config, smoke.tools)
		default:
			err = fmt.Errorf("unknown provider: %s", config.Provider)
		}

		if err != nil {
			return nil, fmt.Errorf("%w: %w", ErrOptions, err)
		}

		if llm.RequiresSessionSystem() {
			smoke.session.AddMessage(llms.SimpleMessage(llms.RoleSystem, smoke.session.SystemMessage))
		}

		smoke.llm = llm

		return smoke, nil
	}
}
