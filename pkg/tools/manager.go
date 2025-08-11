package tools

import (
	"fmt"
	"log/slog"
)

const (
	ToolCreateDirectory = "create_directory"
	ToolGitDiff         = "git_diff"
	ToolGrep            = "grep"
	ToolLint            = "lint"
	ToolListFiles       = "list_files"
	ToolReadFile        = "read_file"
	ToolReplaceLines    = "replace_lines"
	ToolWriteFile       = "write_file"
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
			&GrepTool{ProjectPath: projectPath},
			&LintTool{ProjectPath: projectPath},
			&ListFilesTool{ProjectPath: projectPath},
			&ReadFileTool{ProjectPath: projectPath},
			&ReplaceLinesTool{ProjectPath: projectPath},
			&WriteFileTool{ProjectPath: projectPath},
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
