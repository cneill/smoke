package tools

import (
	"context"
	"fmt"
	"log/slog"
)

// Manager holds the [Tools] that are available for use by the LLM. It makes tool calls and logs results.
// TODO: standard / per-tool timeout for Run() calls
type Manager struct {
	logger      *slog.Logger
	ProjectPath string

	Tools []Tool
}

func NewManager(projectPath, sessionName string) *Manager {
	return &Manager{
		logger:      slog.Default().WithGroup("tools_manager"),
		ProjectPath: projectPath,
		Tools: []Tool{
			&CreateDirectoryTool{ProjectPath: projectPath},
			&EditPlanTool{ProjectPath: projectPath, SessionName: sessionName},
			&GitDiffTool{ProjectPath: projectPath},
			&GoASTTool{ProjectPath: projectPath},
			&GoFumptTool{ProjectPath: projectPath},
			&GoImportsTool{ProjectPath: projectPath},
			&GoLintTool{ProjectPath: projectPath},
			&GoTestTool{ProjectPath: projectPath},
			&GrepTool{ProjectPath: projectPath},
			&ListFilesTool{ProjectPath: projectPath},
			&ReadFileTool{ProjectPath: projectPath},
			&RemovePlanTool{ProjectPath: projectPath, SessionName: sessionName},
			// &ReplaceLinesTool{ProjectPath: projectPath},
			&ReplaceLinesV2Tool{ProjectPath: projectPath},
			&WriteFileTool{ProjectPath: projectPath},
		},
	}
}

func (m *Manager) Params(toolName string) (Params, error) {
	for _, tool := range m.Tools {
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

	for _, tool := range m.Tools {
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
