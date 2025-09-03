package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"slices"
	"strings"
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
		// NewRemovePlanTool,
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

func (m *Manager) GetParams(toolName string) (Params, error) {
	for _, tool := range m.tools {
		if tool.Name() == toolName {
			return tool.Params(), nil
		}
	}

	return Params{}, ErrUnknownTool
}

// GetArgs takes the raw JSON bytes provided in the [llms.LLM] tool call, decodes them into an [Args] map, and validates
// that 1) all required keys are present, 2) unknown keys are not present, 3) value types match those expected for the
// corresponding [Param].
func (m *Manager) GetArgs(toolName string, input []byte) (Args, error) {
	params, err := m.GetParams(toolName)
	if err != nil {
		return nil, fmt.Errorf("failed to get params for tool %q: %w", toolName, err)
	}

	result := Args{}

	decoder := json.NewDecoder(bytes.NewReader(input))
	decoder.UseNumber()

	if err := decoder.Decode(&result); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvalidJSON, err)
	}

	allParamKeys := params.Keys()
	seenKeys := []string{}
	unknownKeys := []string{}

	for key := range result {
		seenKeys = append(seenKeys, key)

		if !slices.Contains(allParamKeys, key) {
			unknownKeys = append(unknownKeys, key)
		}
	}

	if len(unknownKeys) > 0 {
		return nil, fmt.Errorf("%w: %s", ErrUnknownKeys, strings.Join(unknownKeys, ", "))
	}

	missingKeys := []string{}

	for _, key := range params.RequiredKeys() {
		if !slices.Contains(seenKeys, key) {
			missingKeys = append(missingKeys, key)
		}
	}

	if len(missingKeys) > 0 {
		return nil, fmt.Errorf("%w: %s", ErrMissingKeys, strings.Join(missingKeys, ", "))
	}

	if err := result.checkTypes(params); err != nil {
		return nil, err
	}

	return result, nil
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
