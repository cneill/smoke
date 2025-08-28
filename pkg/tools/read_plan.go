package tools

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"

	smokefs "github.com/cneill/smoke/pkg/fs"
)

const (
	ReadPlanStartLine = "start_line"
	ReadPlanEndLine   = "end_line"
)

type ReadPlanTool struct {
	ProjectPath  string
	SessionName  string
	PlanFileName string
}

func NewReadPlanTool(projectPath, sessionName string) Tool {
	return &ReadPlanTool{
		ProjectPath:  projectPath,
		SessionName:  sessionName,
		PlanFileName: sessionName + "_plan.md",
	}
}

func (r *ReadPlanTool) Name() string { return ToolReadPlan }
func (r *ReadPlanTool) Description() string {
	examples := CollectExamples(r.Examples()...)
	return "Read the contents of the plan file, either the whole file or specific lines." + examples
}

func (r *ReadPlanTool) Examples() Examples {
	return Examples{
		{
			Description: `Read the whole plan file, if it exists`,
			Args:        Args{},
		},
		{
			Description: `Read the first 20 lines of the plan file`,
			Args: Args{
				ReadPlanStartLine: 1,
				ReadPlanEndLine:   20,
			},
		},
	}
}

func (r *ReadPlanTool) Params() Params {
	return Params{
		{
			Key:         ReadPlanStartLine,
			Description: "The starting line number to read (1 by default)",
			Type:        ParamTypeNumber,
			Required:    false,
		},
		{
			Key:         ReadPlanEndLine,
			Description: "The last line number to read (end of file by default)",
			Type:        ParamTypeNumber,
			Required:    false,
		},
	}
}

func (r *ReadPlanTool) Run(ctx context.Context, args Args) (string, error) {
	fullPath, err := smokefs.GetRelativePath(r.ProjectPath, r.PlanFileName)
	if err != nil {
		return "", fmt.Errorf("%w: invalid session name / plan path: %w", ErrArguments, err)
	}

	_, statErr := os.Stat(fullPath)
	if statErr != nil && errors.Is(statErr, fs.ErrNotExist) {
		return "", fmt.Errorf("%w: plan file does not yet exist - create with the %q tool", ErrFileSystem, ToolEditPlan)
	}

	childArgs := Args{
		ReadFilePath: fullPath,
	}

	if startLine := args.GetInt(ReadPlanStartLine); startLine != nil {
		childArgs[ReadFileStartLine] = *startLine
	}

	if endLine := args.GetInt(ReadPlanEndLine); endLine != nil {
		childArgs[ReadFileEndLine] = *endLine
	}

	readTool := &ReadFileTool{ProjectPath: r.ProjectPath}

	return readTool.Run(ctx, childArgs)
}
