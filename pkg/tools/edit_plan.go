package tools

import (
	"context"
	"fmt"
	"os"

	"github.com/cneill/smoke/pkg/fs"
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

var _ = Tool(&EditPlanTool{})

func (e *EditPlanTool) Name() string { return ToolEditPlan }

func (e *EditPlanTool) Description() string {
	return fmt.Sprintf(
		"Edit ONLY the session plan file %q. Replace the content between lines %q and %q with the contents in %q. Line "+
			"numbers are 1-indexed, as they are in the output values of `read_file`, `grep`, etc. and should match "+
			"those values. 0/0 can be used to initialize the file's contents.",
		e.SessionName+"_plan.md", EditPlanStartLine, EditPlanEndLine, EditPlanReplace,
	)
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

	fullPath, err := fs.GetRelativePath(e.ProjectPath, planFileName)
	if err != nil {
		return "", fmt.Errorf("%w: invalid session name / plan path: %w", ErrArguments, err)
	}

	if _, err := os.Stat(fullPath); err != nil {
		return "", fmt.Errorf("%w: could not stat %s in root directory: %w; may need to be created with write_file", ErrFileSystem, planFileName, err)
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

	// Delegate the edit to ReplaceLinesV2Tool for consistent, tested behavior
	replaceTool := &ReplaceLinesV2Tool{ProjectPath: e.ProjectPath}

	childArgs := Args{
		ReplaceLinesV2Path:      planFileName,
		ReplaceLinesV2StartLine: *startLine,
		ReplaceLinesV2EndLine:   *endLine,
		ReplaceLinesV2Replace:   *replace,
	}

	return replaceTool.Run(ctx, childArgs)
}
