package tools

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/cneill/smoke/pkg/fs"
)

const (
	GitDiffStat = "stat"
	GitDiffPath = "path"
)

type GitDiffTool struct {
	ProjectPath string
}

func NewGitDiffTool(projectPath, _ string) Tool {
	return &GitDiffTool{ProjectPath: projectPath}
}

func (g *GitDiffTool) Name() string { return ToolGitDiff }
func (g *GitDiffTool) Description() string {
	examples := CollectExamples(g.Examples()...)

	return fmt.Sprintf("Check the git diff in the ProjectPath. Optionally, provide %q as true to include basic "+
		"statistics. Optionally, specify %q to diff a specific file/directory.%s",
		GitDiffStat, GitDiffPath, examples)
}

func (g *GitDiffTool) Examples() Examples {
	return Examples{
		{
			Description: `Get stats about the diff of the whole repository`,
			Args:        Args{GitDiffStat: true},
		},
		{
			Description: `Diff the "pkg/main.go" file specifically.`,
			Args:        Args{GitDiffPath: "pkg/main.go"},
		},
	}
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
			Key:         GitDiffPath,
			Description: "Diff a specific file/directory",
			Type:        ParamTypeString,
			Required:    false,
		},
	}
}

func (g *GitDiffTool) Run(ctx context.Context, args Args) (string, error) {
	fullPath := g.ProjectPath

	if path := args.GetString(GitDiffPath); path != nil && *path != "" {
		if strings.ContainsAny(*path, "`$%&;[](){}| ") {
			return "", fmt.Errorf("%w: path contained invalid characters that might allow command execution", ErrArguments)
		}

		relPath, err := fs.GetRelativePath(g.ProjectPath, *path)
		if err != nil {
			return "", fmt.Errorf("%w: path error: %w", ErrArguments, err)
		}

		fullPath = relPath
	}

	params := []string{
		"diff",
		"--no-color",
	}

	if stat := args.GetBool(GitDiffStat); stat != nil && *stat {
		params = append(params, "--stat")
	}

	params = append(params, fullPath)

	ctx, cancel := context.WithTimeout(ctx, time.Second*3)
	defer cancel()

	output, err := exec.CommandContext(ctx, "git", params...).CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("error executing git diff: %w", err)
	}

	return string(output), nil
}
