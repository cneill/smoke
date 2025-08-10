package tools

import (
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

const (
	GitDiffStat = "stat"
	GitDiffFile = "file"
)

type GitDiffTool struct {
	ProjectPath string
}

func (g *GitDiffTool) Name() string { return ToolGitDiff }

func (g *GitDiffTool) Description() string {
	return "Check the git diff in the ProjectPath."
}

func (g *GitDiffTool) Params() Params {
	return Params{
		{
			Key:         GitDiffStat,
			Description: "Provide basic statistics about the git diff",
			Type:        ParamTypeBoolean,
			Required:    false,
		},
		{
			Key:         GitDiffFile,
			Description: "Diff a specific file",
			Type:        ParamTypeString,
			Required:    false,
		},
	}
}

func (g *GitDiffTool) Run(args Args) (string, error) {
	path, err := filepath.Abs(g.ProjectPath)
	if err != nil {
		return "", fmt.Errorf("invalid path: %w", err)
	}

	if strings.ContainsAny(path, "`$%&;[](){}| ") {
		return "", errors.New("path contained invalid characters that might allow command execution")
	}

	params := []string{
		"-C", path,
		"diff",
	}

	if stat := args.GetBool(GitDiffStat); stat != nil && *stat {
		params = append(params, "--stat")
	}

	if file := args.GetString(GitDiffFile); file != nil && *file != "" {
		params = append(params, *file)
	}

	cmd := exec.Command("git", params...)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("error executing git diff: %w", err)
	}

	return string(output), nil
}
