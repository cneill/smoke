package tools

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
)

// Manager holds the [Tools] that are available for use by the LLM. It makes tool calls and logs results.
// TODO: standard / per-tool timeout for Run() calls
type Manager struct {
	logger      *slog.Logger
	ProjectPath string
	SessionName string

	tools     Tools
	toolMutex sync.RWMutex
}

func AllTools() []Initializer {
	return []Initializer{
		NewCreateDirectoryTool,
		NewEditPlanTool,
		NewGitDiffTool,
		NewGoASTTool,
		NewGoFumptTool,
		NewGoImportsTool,
		NewGoLintTool,
		NewGoTestTool,
		NewGrepTool,
		NewListFilesTool,
		NewReadFileTool,
		NewReadPlanTool,
		NewRemovePlanTool,
		NewReplaceLinesTool,
		// NewSummarizeHistoryTool,
		NewWriteFileTool,
	}
}

func PlanningTools() []Initializer {
	return []Initializer{
		NewEditPlanTool,
		NewGitDiffTool,
		NewGoASTTool,
		NewGoLintTool,
		NewGoTestTool,
		NewGrepTool,
		NewListFilesTool,
		NewReadFileTool,
		NewReadPlanTool,
		// NewSummarizeHistoryTool,
	}
}

func NewManager(projectPath, sessionName string) *Manager {
	manager := &Manager{
		logger:      slog.Default().WithGroup("tools_manager"),
		ProjectPath: projectPath,
		SessionName: sessionName,

		toolMutex: sync.RWMutex{},
	}

	manager.SetTools(AllTools()...)

	return manager
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
		tools = append(tools, init(m.ProjectPath, m.SessionName))
	}

	m.tools = tools

	slog.Debug("setting tools", "tools", m.tools.Names())
}

func (m *Manager) Params(toolName string) (Params, error) {
	for _, tool := range m.tools {
		if tool.Name() == toolName {
			return tool.Params(), nil
		}
	}

	return Params{}, ErrUnknownTool
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
