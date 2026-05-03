package tools

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"github.com/cneill/smoke/internal/uimsg"
	"github.com/cneill/smoke/pkg/elicit"
	"github.com/cneill/smoke/pkg/llmctx/skills"
	"github.com/cneill/smoke/pkg/plan"
)

type ManagerOpts struct {
	ProjectPath      string
	SessionName      string
	ToolInitializers []Initializer
	PlanManager      *plan.Manager
	SkillCatalog     skills.Catalog
	ElicitManager    *elicit.Manager
}

func (m *ManagerOpts) OK() error {
	switch {
	case m.ProjectPath == "":
		return fmt.Errorf("missing project path")
	case m.SessionName == "":
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

	initializers  []Initializer
	tools         Tools
	toolMutex     sync.RWMutex
	planManager   *plan.Manager
	skillCatalog  skills.Catalog
	elicitManager *elicit.Manager

	teaEmitter uimsg.TeaEmitter
}

func NewManager(opts *ManagerOpts) (*Manager, error) {
	if err := opts.OK(); err != nil {
		return nil, fmt.Errorf("error with tool manager options: %w", err)
	}

	manager := &Manager{
		logger:      slog.Default().WithGroup("tools_manager"),
		ProjectPath: opts.ProjectPath,
		SessionName: opts.SessionName,

		initializers:  opts.ToolInitializers,
		toolMutex:     sync.RWMutex{},
		planManager:   opts.PlanManager,
		skillCatalog:  opts.SkillCatalog,
		elicitManager: opts.ElicitManager,
	}

	if opts.ToolInitializers != nil {
		tools := manager.InitTools(opts.ToolInitializers...)
		manager.SetTools(tools...)
	} else {
		manager.tools = Tools{}
	}

	return manager, nil
}

func (m *Manager) SetTeaEmitter(emitter uimsg.TeaEmitter) {
	m.teaEmitter = emitter

	m.toolMutex.Lock()
	defer m.toolMutex.Unlock()

	for _, tool := range m.tools {
		// We have to do this here because the tea emitter is injected later than initial startup.
		if wte, ok := tool.(WantsTeaEmitter); ok && m.teaEmitter != nil {
			slog.Debug("Setting tea emitter for tool", "name", tool.Name())
			wte.SetTeaEmitter(m.teaEmitter)
		}
	}
}

func (m *Manager) GetTools() Tools {
	m.toolMutex.RLock()
	defer m.toolMutex.RUnlock()

	return m.tools
}

func (m *Manager) InitTools(initializers ...Initializer) Tools {
	tools := Tools{}

	for _, init := range initializers {
		tool, err := init(m.ProjectPath, m.SessionName)
		if err != nil {
			m.logger.Error("tool initializer failed", "error", err)
			continue
		}

		if wpm, ok := tool.(WantsPlanManager); ok {
			wpm.SetPlanManager(m.planManager)
		}

		if wsc, ok := tool.(WantsSkillCatalog); ok {
			wsc.SetSkillCatalog(m.skillCatalog)
		}

		if wem, ok := tool.(WantsElicitManager); ok && m.elicitManager != nil {
			wem.SetElicitManager(m.elicitManager)
		}

		tools = append(tools, tool)
	}

	slog.Debug("initialized tools", "tools", tools.Names())

	return tools
}

func (m *Manager) SetTools(tools ...Tool) {
	m.toolMutex.Lock()
	defer m.toolMutex.Unlock()

	m.tools = tools
	slog.Debug("set manager tools", "tools", Tools(tools).Names())
}

func (m *Manager) AddTools(tools ...Tool) {
	m.toolMutex.Lock()
	defer m.toolMutex.Unlock()

	m.tools = append(m.tools, tools...)
	slog.Debug("adding manager tools", "tools", Tools(tools).Names())
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
func (m *Manager) CallTool(ctx context.Context, toolName string, args Args) (*Output, error) {
	m.logger.Debug("calling tool", "tool_name", toolName, "args", args)

	for _, tool := range m.tools {
		if tool.Name() == toolName {
			output, err := tool.Run(ctx, args)
			if err != nil {
				m.logger.Error("tool call unsuccessful", "tool_name", toolName, "args", args, "output", output, "error", err)
				return nil, fmt.Errorf("%w: %w", ErrCallFailed, err)
			}

			m.logger.Debug("tool call successful", "tool_name", toolName, "args", args, "output", output)

			return output, nil
		}
	}

	m.logger.Error("unknown tool", "tool_name", toolName)

	return nil, ErrUnknownTool
}

func (m *Manager) Teardown() error {
	if m.planManager != nil {
		if err := m.planManager.Teardown(); err != nil {
			return fmt.Errorf("failed teardown of plan manager: %w", err)
		}
	}

	return nil
}
