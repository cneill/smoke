package tools

import (
	"log/slog"
	"os/exec"
)

// GofmtTool is a tool to run gofmt on the project directory.
type GofmtTool struct {
	ProjectPath string
}

// Name returns the name of the tool.
func (t *GofmtTool) Name() string {
	return "gofmt"
}

// Description returns the description of the tool.
func (t *GofmtTool) Description() string {
	return "Formats Go source code using gofmt."
}

// Params returns the required parameters for this tool.
func (t *GofmtTool) Params() Params {
	return Params{}
}

// Run executes gofmt and returns its output.
func (t *GofmtTool) Run(_ Args) (string, error) {
	path := t.ProjectPath + "/..."
	cmd := exec.Command("gofmt", "-w", path)

	output, err := cmd.CombinedOutput()
	if err != nil {
		slog.Warn("error from gofmt execution", "path", path, "error", err)
	}

	return string(output), nil
}
