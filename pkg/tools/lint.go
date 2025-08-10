package tools

import (
	"fmt"
	"log/slog"
	"os"
	"os/exec"

	"github.com/cneill/smoke/pkg/utils"
)

const (
	LintPath = "path"
)

type LintTool struct {
	ProjectPath string
}

func (l *LintTool) Name() string {
	return "lint"
}

func (l *LintTool) Description() string {
	return "Runs the golangci-lint linter against the specified file/directory, or the whole project directory if a " +
		"path is not specified."
}

func (l *LintTool) Params() Params {
	return Params{
		{
			Key:         LintPath,
			Description: "The path of the directory/file to lint",
			Type:        ParamTypeString,
			Required:    false,
		},
	}
}

func (l *LintTool) Run(args Args) (string, error) {
	fullPath := l.ProjectPath

	if path := args.GetString(LintPath); path != nil {
		relPath, err := utils.GetRelativePath(l.ProjectPath, *path)
		if err != nil {
			return "", fmt.Errorf("path error: %w", err)
		}

		fullPath = relPath
	}

	stat, err := os.Stat(fullPath)
	if err != nil {
		return "", fmt.Errorf("failed to stat path %s: %w", fullPath, err)
	}

	if stat.IsDir() {
		fullPath += "/..."
	}

	cmd := exec.Command("golangci-lint", "run", fullPath)
	cmd.Dir = l.ProjectPath

	output, err := cmd.CombinedOutput()
	if err != nil {
		slog.Warn("error from golangci-lint execution", "path", fullPath, "error", err)
	}

	return string(output), nil
}
