package tools

import (
	"fmt"
	"os"
	"path/filepath"
)

type RemovePlanTool struct {
	ProjectPath string
}

var _ = Tool(&RemovePlanTool{})

func (r *RemovePlanTool) Name() string { return ToolRemovePlan }
func (r *RemovePlanTool) Description() string {
	return "Remove the plan file when the plan has been completed"
}

func (r *RemovePlanTool) Params() Params { return Params{} }

func (r *RemovePlanTool) Run(_ Args) (string, error) {
	path := filepath.Join(r.ProjectPath, "smoke_plan.md")
	if _, err := os.Stat(path); err != nil {
		return "", fmt.Errorf("%w: could not stat smoke_plan.md in root directory: %w", ErrFileSystem, err)
	}

	if err := os.Remove(path); err != nil {
		return "", fmt.Errorf("%w: failed to remove plan file smoke_plan.md: %w", ErrFileSystem, err)
	}

	return "Removed plan file smoke_model.md", nil
}
