package tools

import (
	"fmt"
	"log/slog"
)

// Manager holds the [Tools] that are available for use by the LLM. It makes tool calls and logs results.
type Manager struct {
	logger      *slog.Logger
	ProjectPath string

	Tools Tools
}

func NewManager(projectPath string) *Manager {
	return &Manager{
		logger:      slog.Default().WithGroup("tools_manager"),
		ProjectPath: projectPath,
		Tools: Tools{
			&CreateDirectoryTool{ProjectPath: projectPath},
			&GitDiffTool{ProjectPath: projectPath},
			&GoASTTool{ProjectPath: projectPath},
			&GoFumptTool{ProjectPath: projectPath},
			&GoImportsTool{ProjectPath: projectPath},
			&GoLintTool{ProjectPath: projectPath},
			&GoTestTool{ProjectPath: projectPath},
			&GrepTool{ProjectPath: projectPath},
			&ListFilesTool{ProjectPath: projectPath},
			&ReadFileTool{ProjectPath: projectPath},
			&RemovePlanTool{ProjectPath: projectPath},
			&ReplaceLinesTool{ProjectPath: projectPath},
			&WriteFileTool{ProjectPath: projectPath},
		},
	}
}

func (m *Manager) CallTool(toolName string, args Args) (string, error) {
	m.logger.Debug("calling tool", "tool_name", toolName, "args", args)

	output, err := m.Tools.Call(toolName, args)
	if err != nil {
		m.logger.Debug("tool call unsuccessful", "tool_name", toolName, "args", args, "output", output, "error", err)
		return output, fmt.Errorf("%s: %w", toolName, err)
	}

	m.logger.Debug("tool call successful", "tool_name", toolName, "args", args, "output", output)

	return output, nil
}
