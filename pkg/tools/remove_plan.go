package tools

import (
	"context"
	"fmt"
	"os"

	"github.com/cneill/smoke/pkg/fs"
)

type RemovePlanTool struct {
	ProjectPath string
	SessionName string
}

func NewRemovePlanTool(projectPath, sessionName string) Tool {
	return &RemovePlanTool{ProjectPath: projectPath, SessionName: sessionName}
}

func (r *RemovePlanTool) Name() string { return ToolRemovePlan }
func (r *RemovePlanTool) Description() string {
	examples := CollectExamples(r.Examples()...)

	return "Remove the plan file when the plan has been completed." + examples
}

func (r *RemovePlanTool) Examples() Examples {
	return Examples{
		{
			Description: "Remove the plan file",
			Args:        Args{},
		},
	}
}

func (r *RemovePlanTool) Params() Params { return Params{} }

func (r *RemovePlanTool) Run(_ context.Context, _ Args) (string, error) {
	planFileName := r.SessionName + "_plan.md"

	path, err := fs.GetRelativePath(r.ProjectPath, planFileName)
	if err != nil {
		return "", fmt.Errorf("invalid session name / plan path: %w", err)
	}

	if _, err := os.Stat(path); err != nil {
		return "", fmt.Errorf("%w: could not stat %s in root directory: %w", ErrFileSystem, planFileName, err)
	}

	if err := os.Remove(path); err != nil {
		return "", fmt.Errorf("%w: failed to remove plan file %s: %w", ErrFileSystem, planFileName, err)
	}

	return "Removed plan file " + planFileName, nil
}
