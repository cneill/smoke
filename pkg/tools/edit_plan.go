package tools

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"

	smokefs "github.com/cneill/smoke/pkg/fs"
)

const (
	EditPlanStartLine = "start_line"
	EditPlanEndLine   = "end_line"
	EditPlanReplace   = "replace"
)

type EditPlanTool struct {
	ProjectPath string
	SessionName string
}

func NewEditPlanTool(projectPath, sessionName string) Tool {
	return &EditPlanTool{
		ProjectPath: projectPath,
		SessionName: sessionName,
	}
}

func (e *EditPlanTool) Name() string { return ToolEditPlan }
func (e *EditPlanTool) Description() string {
	sessionFile := e.SessionName + "_plan.md"
	examples := CollectExamples(e.Examples()...)

	return fmt.Sprintf("Edit ONLY the session plan file %q. Replace the content between lines %q and %q with the "+
		"contents in %q. Line numbers are 1-indexed, as they are in the output values of %q, %q, etc. "+
		"and should match those values. This calls `replace_lines` under the hood, so expect similar results.%s",
		sessionFile, EditPlanStartLine, EditPlanEndLine,
		EditPlanReplace, ToolGrep, ToolReadFile,
		examples,
	)
}

func (e *EditPlanTool) Examples() Examples {
	return Examples{
		{
			Description: "Populate the initial lines of the plan file. Will create if it doesn't exist",
			Args: Args{
				EditPlanStartLine: 0,
				EditPlanEndLine:   0,
				EditPlanReplace:   "## New plan\nTask 1: Do thing",
			},
		},
		{
			Description: "Replace the first 2 lines in the plan file",
			Args: Args{
				EditPlanStartLine: 1,
				EditPlanEndLine:   2,
				EditPlanReplace:   "# Plan to do Y\nFirst do A...",
			},
		},
	}
}

func (e *EditPlanTool) Params() Params {
	return Params{
		{
			Key:         EditPlanStartLine,
			Description: fmt.Sprintf("The first line to replace with the text in %q", EditPlanReplace),
			Type:        ParamTypeNumber,
			Required:    true,
		},
		{
			Key:         EditPlanEndLine,
			Description: fmt.Sprintf("The last line to replace with the text in %q", EditPlanReplace),
			Type:        ParamTypeNumber,
			Required:    true,
		},
		{
			Key:         EditPlanReplace,
			Description: "The string content that will replace the lines specified by the line numbers in start_line and end_line",
			Type:        ParamTypeString,
			Required:    true,
		},
	}
}

func (e *EditPlanTool) Run(ctx context.Context, args Args) (string, error) {
	planFileName := e.SessionName + "_plan.md"

	fullPath, err := smokefs.GetRelativePath(e.ProjectPath, planFileName)
	if err != nil {
		return "", fmt.Errorf("%w: invalid session name / plan path: %w", ErrArguments, err)
	}

	_, statErr := os.Stat(fullPath)
	if statErr != nil && errors.Is(statErr, fs.ErrNotExist) {
		file, err := os.Create(fullPath)
		if err != nil {
			slog.Error("unable to create session plan file", "error", err)
			return "", fmt.Errorf("%w: failed to create new session file %s: %w", ErrFileSystem, planFileName, err)
		}

		slog.Debug("created session plan file", "path", fullPath)

		file.Close()
	}

	startLine := args.GetInt(EditPlanStartLine)
	endLine := args.GetInt(EditPlanEndLine)
	replace := args.GetString(EditPlanReplace)

	switch {
	case startLine == nil || endLine == nil || replace == nil:
		return "", fmt.Errorf("%w: missing %q, %q, or %q", ErrArguments, EditPlanStartLine, EditPlanEndLine, EditPlanReplace)
	case *startLine < 0 || *endLine < 0:
		return "", fmt.Errorf("%w: %q or %q is less than 0", ErrArguments, EditPlanStartLine, EditPlanEndLine)
	case *startLine > *endLine:
		return "", fmt.Errorf("%w: %q is greater than %q", ErrArguments, EditPlanStartLine, EditPlanEndLine)
	}

	// Delegate the edit to ReplaceLinesTool for consistent, tested behavior
	replaceTool := &ReplaceLinesTool{ProjectPath: e.ProjectPath}

	childArgs := Args{
		ReplaceLinesPath:      planFileName,
		ReplaceLinesStartLine: *startLine,
		ReplaceLinesEndLine:   *endLine,
		ReplaceLinesReplace:   *replace,
	}

	return replaceTool.Run(ctx, childArgs)
}
