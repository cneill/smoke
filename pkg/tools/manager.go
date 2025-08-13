package tools

import (
	"fmt"
	"log/slog"
)

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
			&GoFumptTool{ProjectPath: projectPath},
			&GoLintTool{ProjectPath: projectPath},
			&GoTestTool{ProjectPath: projectPath},
			&GrepTool{ProjectPath: projectPath},
			&ListFilesTool{ProjectPath: projectPath},
			&ReadFileTool{ProjectPath: projectPath},
			&ReplaceLinesTool{ProjectPath: projectPath},
			&WriteFileTool{ProjectPath: projectPath},
		},
	}
}

func (m *Manager) CallTool(toolName string, args Args) (string, error) {
	m.logger.Debug("calling tool", "tool_name", toolName, "args", args)

	output, err := m.Tools.Call(toolName, args)
	if err != nil {
		return output, fmt.Errorf("%w: %s: %w", ErrCallFailed, toolName, err)
	}

	m.logger.Debug("tool call successful", "tool_name", toolName, "args", args, "output", output)

	return output, nil
}
