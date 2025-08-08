package tools

import (
	"errors"
	"fmt"
	"log/slog"
)

const (
	ToolListFiles = "list_files"
	ToolReadFile  = "read_file"
	ToolWriteFile = "write_file"
)

var (
	ErrArguments   = errors.New("arguments error")
	ErrCallFailed  = errors.New("tool call failed")
	ErrUnknownTool = errors.New("unknown tool")
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
			&ListFilesTool{ProjectPath: projectPath},
			&ReadFileTool{ProjectPath: projectPath},
			&WriteFileTool{ProjectPath: projectPath},
			&GrepTool{ProjectPath: projectPath},
			&CreateDirectoryTool{ProjectPath: projectPath},
			&LintTool{ProjectPath: projectPath},
			&GitDiffTool{ProjectPath: projectPath},
			&ReplaceLinesTool{ProjectPath: projectPath},
			// &GofmtTool{ProjectPath: projectPath},
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
