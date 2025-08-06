package tools

import (
	"log/slog"
	"os/exec"
)

// LintTool is a tool to run golangci-lint on the project directory.
type LintTool struct {
	ProjectPath string
}

// Name returns the name of the tool.
func (t *LintTool) Name() string {
	return "lint"
}

// Description returns the description of the tool.
func (t *LintTool) Description() string {
	return "Runs golangci-lint against the ProjectDirectory."
}

// Params returns the required parameters for this tool.
func (t *LintTool) Params() Params {
	return Params{}
}

// Run executes golangci-lint and returns its output.
func (t *LintTool) Run(_ Args) (string, error) {
	path := t.ProjectPath + "/..."
	cmd := exec.Command("golangci-lint", "run", path)

	output, err := cmd.CombinedOutput()
	if err != nil {
		slog.Warn("error from golangci-lint execution", "path", path, "error", err)
	}

	return string(output), nil
}
