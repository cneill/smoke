package smoke

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/cneill/smoke/pkg/config"
	"github.com/cneill/smoke/pkg/llms"
	"github.com/cneill/smoke/pkg/mcp"
)

// OptFunc is used to configure aspects of Smoke.
type OptFunc func(smoke *Smoke) (*Smoke, error)

// WithProjectPath sets the directory we'll work from.
func WithProjectPath(path string) OptFunc {
	return func(smoke *Smoke) (*Smoke, error) {
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

		return smoke, nil
	}
}

func WithConfig(config *config.Config) OptFunc {
	return func(smoke *Smoke) (*Smoke, error) {
		smoke.config = config
		return smoke, nil
	}
}

// WithSessionInfo configures the details of the session we'll work with.
func WithSessionInfo(name, systemPrompt string) OptFunc {
	return func(smoke *Smoke) (*Smoke, error) {
		smoke.sessionMutex.Lock()
		defer smoke.sessionMutex.Unlock()

		smoke.mainSessionName = name
		smoke.mainSystemPrompt = systemPrompt

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

		smoke.llmConfig = config

		return smoke, nil
	}
}

// WithMCPClient adds an MCP client to Smoke. Its tools will be added to the tool manager attached to Smoke's session.
func WithMCPClient(client *mcp.CommandClient) OptFunc {
	return func(smoke *Smoke) (*Smoke, error) {
		smoke.mcpClients = append(smoke.mcpClients, client)
		return smoke, nil
	}
}

// WithTeaEmitter allows us to inject messages straight into Bubbletea's event loop rather than round-tripping.
func WithTeaEmitter(emitter TeaEmitter) OptFunc {
	return func(smoke *Smoke) (*Smoke, error) {
		smoke.teaEmitter = emitter
		return smoke, nil
	}
}
