package smoke

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/cneill/smoke/internal/uimsg"
	"github.com/cneill/smoke/pkg/config"
	"github.com/cneill/smoke/pkg/llms"

	git "github.com/go-git/go-git/v5"
)

// OptFunc is used to configure aspects of Smoke.
type OptFunc func(smoke *Smoke) (*Smoke, error)

// WithProjectPath sets the directory we'll work from.
func WithProjectPath(path string, noGit bool) OptFunc {
	return func(smoke *Smoke) (*Smoke, error) {
		absPath, err := filepath.Abs(path)
		if err != nil {
			return nil, fmt.Errorf("invalid project path %q: %w", path, err)
		}

		if _, err := os.Stat(absPath); err != nil {
			return nil, fmt.Errorf("failed to stat project path %q: %w", absPath, err)
		}

		// Resolve symlinks so the project path is canonical regardless of noGit, and so it can be compared against the
		// git worktree root (which go-git returns with symlinks resolved).
		absPath, err = filepath.EvalSymlinks(absPath)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve symlinks in project path %q: %w", path, err)
		}

		if !noGit {
			repo, err := git.PlainOpenWithOptions(absPath, &git.PlainOpenOptions{DetectDotGit: true})
			if err != nil {
				return nil, fmt.Errorf("failed to find git repository in project path %q: %w", absPath, err)
			}

			wt, err := repo.Worktree()
			if err != nil {
				return nil, fmt.Errorf("failed to open git repository worktree in project path %q: %w", absPath, err)
			}

			gitRoot := wt.Filesystem.Root()

			gitRootAbs, err := filepath.Abs(gitRoot)
			if err != nil {
				return nil, fmt.Errorf("git repository root %q is invalid: %w", gitRoot, err)
			}

			if gitRootAbs != absPath {
				slog.Info("using root of git repository as project path", "supplied_path", path, "repo_root", gitRootAbs)

				absPath = gitRootAbs
			}
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

// WithOneShot configures Smoke for a single non-interactive, read-only request. Plans, skills, the ask manager, MCP
// clients, and commands are not set up.
func WithOneShot() OptFunc {
	return func(smoke *Smoke) (*Smoke, error) {
		smoke.oneShot = true
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
