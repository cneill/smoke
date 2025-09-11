package tools

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"sync"

	"github.com/cneill/smoke/pkg/fs"
	"github.com/cneill/smoke/pkg/plan"
)

type ManagerOpts struct {
	ProjectPath     string
	SessionName     string
	Tools           []Initializer
	WithPlanManager bool
}

func (m *ManagerOpts) OK() error {
	if m.ProjectPath == "" {
		return fmt.Errorf("missing project path")
	}

	if m.SessionName == "" {
		return fmt.Errorf("missing session name")
	}

	return nil
}

// Manager holds the [Tools] that are available for use by the LLM. It makes tool calls and logs results.
// TODO: standard / per-tool timeout for Run() calls
type Manager struct {
	logger      *slog.Logger
	ProjectPath string
	SessionName string

	tools       Tools
	toolMutex   sync.RWMutex
	planManager *plan.Manager
	planFile    *os.File
}

func NewManager(opts *ManagerOpts) (*Manager, error) {
	if err := opts.OK(); err != nil {
		return nil, fmt.Errorf("error with tool manager options: %w", err)
	}

	manager := &Manager{
		logger:      slog.Default().WithGroup("tools_manager"),
		ProjectPath: opts.ProjectPath,
		SessionName: opts.SessionName,

		toolMutex: sync.RWMutex{},
	}

	if opts.WithPlanManager {
		planFileName := opts.SessionName + "_plan.json"

		relPath, err := fs.GetRelativePath(opts.ProjectPath, planFileName)
		if err != nil {
			return nil, fmt.Errorf("invalid session plan file path (%s): %w", planFileName, err)
		}

		// TODO: stat for existing plan planFile, create the manager by loading if exists
		planFile, err := os.OpenFile(relPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
		if err != nil {
			return nil, fmt.Errorf("failed to open session plan file: %w", err)
		}

		manager.planFile = planFile
		manager.planManager = plan.NewManager(planFile)
	}

	if opts.Tools != nil {
		manager.SetTools(opts.Tools...)
	} else {
		manager.SetTools(AllTools()...)
	}

	return manager, nil
}

func (m *Manager) GetTools() Tools {
	m.toolMutex.RLock()
	defer m.toolMutex.RUnlock()

	return m.tools
}

func (m *Manager) SetTools(initializers ...Initializer) {
	m.toolMutex.Lock()
	defer m.toolMutex.Unlock()

	tools := Tools{}
	for _, init := range initializers {
		tool := init(m.ProjectPath, m.SessionName)
		if pt, ok := tool.(PlanTool); ok {
			pt.SetPlanManager(m.planManager)
		}

		tools = append(tools, tool)
	}

	m.tools = tools

	slog.Debug("setting tools", "tools", m.tools.Names())
}

func (m *Manager) GetParams(toolName string) (Params, error) {
	for _, tool := range m.tools {
		if tool.Name() == toolName {
			return tool.Params(), nil
		}
	}

	return Params{}, ErrUnknownTool
}

// GetArgs takes the raw JSON bytes provided in the [llms.LLM] tool call, decodes them into an [Args] map, and validates
// that 1) all required keys are present, 2) unknown keys are not present, 3) values and value types match those
// expected for the corresponding [Param].
func (m *Manager) GetArgs(toolName string, input []byte) (Args, error) {
	params, err := m.GetParams(toolName)
	if err != nil {
		return nil, fmt.Errorf("failed to get params for tool %q: %w", toolName, err)
	}

	return ParseArgs(params, input)
}

// CallTool finds the [Tool] with the name 'toolName' (if known, otherwise returns ErrUnknownTool), and calls it with
// the provided 'args'. After running, it returns the output or the error returned by Run wrapped with ErrCallFailed.
func (m *Manager) CallTool(ctx context.Context, toolName string, args Args) (string, error) {
	m.logger.Debug("calling tool", "tool_name", toolName, "args", args)

	for _, tool := range m.tools {
		if tool.Name() == toolName {
			output, err := tool.Run(ctx, args)
			if err != nil {
				m.logger.Error("tool call unsuccessful", "tool_name", toolName, "args", args, "output", output, "error", err)
				return "", fmt.Errorf("%w: %w", ErrCallFailed, err)
			}

			m.logger.Debug("tool call successful", "tool_name", toolName, "args", args, "output", output)

			return output, nil
		}
	}

	m.logger.Error("unknown tool", "tool_name", toolName)

	return "", ErrUnknownTool
}
