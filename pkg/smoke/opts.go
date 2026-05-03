package smoke

import (
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/cneill/smoke/internal/uimsg"
	"github.com/cneill/smoke/pkg/config"
	"github.com/cneill/smoke/pkg/llms"
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
			if errors.Is(err, fs.ErrNotExist) {
				return nil, fmt.Errorf("specified project path %q is not a git repository", absPath)
			}

			return nil, fmt.Errorf("failed to stat '.git' directory in project path %q: %w", absPath, err)
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

// WithSessionName configures the name of the main session.
func WithSessionName(name string) OptFunc {
	return func(smoke *Smoke) (*Smoke, error) {
		smoke.sessionMutex.Lock()
		defer smoke.sessionMutex.Unlock()

		smoke.mainSessionName = name

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
// WithProjectPath and WithSessionName.
func WithLLMConfig(config *llms.Config) OptFunc {
	return func(smoke *Smoke) (*Smoke, error) {
		if err := config.OK(); err != nil {
			return nil, fmt.Errorf("LLM config: %w", err)
		}

		smoke.llmConfig = config

		return smoke, nil
	}
}

// WithTeaEmitter allows us to inject messages straight into Bubbletea's event loop rather than round-tripping.
// NOTE: This has to be injected *after* Smoke is first set up, because we need the ui model (which relies on Smoke) to
// be set up before we can get an emitter for the Bubbletea Program running its event loop.
func WithTeaEmitter(emitter uimsg.TeaEmitter) OptFunc {
	return func(smoke *Smoke) (*Smoke, error) {
		smoke.teaEmitter = emitter

		session := smoke.getMainSession()
		if session != nil {
			slog.Debug("SETTING TEA EMITTER ON SESSION TOOLS MANAGER _AFTER_ SETUP")
			session.Tools.SetTeaEmitter(emitter)
		}

		return smoke, nil
	}
}
