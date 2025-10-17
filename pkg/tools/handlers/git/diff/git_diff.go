package gitdiff

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/cneill/smoke/pkg/fs"
	"github.com/cneill/smoke/pkg/tools"
)

const (
	Name = "git_diff"

	ParamStat = "stat"
	ParamPath = "path"
)

type GitDiff struct {
	ProjectPath string
}

func New(projectPath, _ string) (tools.Tool, error) {
	return &GitDiff{ProjectPath: projectPath}, nil
}

func (g *GitDiff) Name() string { return Name }
func (g *GitDiff) Description() string {
	examples := tools.CollectExamples(g.Examples()...)

	return fmt.Sprintf("Check the git diff in the ProjectPath. Optionally, provide %q as true to include basic "+
		"statistics. Optionally, specify %q to diff a specific file/directory.%s",
		ParamStat, ParamPath, examples)
}

func (g *GitDiff) Examples() tools.Examples {
	return tools.Examples{
		{
			Description: `Get stats about the diff of the whole repository`,
			Args:        tools.Args{ParamStat: true},
		},
		{
			Description: `Diff the "pkg/main.go" file specifically.`,
			Args:        tools.Args{ParamPath: "pkg/main.go"},
		},
	}
}

func (g *GitDiff) Params() tools.Params {
	return tools.Params{
		{
			Key:         ParamStat,
			Description: "Provide basic statistics about the git diff",
			Type:        tools.ParamTypeBoolean,
			Required:    false,
		},
		{
			Key:         ParamPath,
			Description: "Diff a specific file/directory",
			Type:        tools.ParamTypeString,
			Required:    false,
		},
	}
}

func (g *GitDiff) Run(ctx context.Context, args tools.Args) (string, error) {
	fullPath := g.ProjectPath

	if path := args.GetString(ParamPath); path != nil && *path != "" {
		if strings.ContainsAny(*path, "`$%&;[](){}| ") {
			return "", fmt.Errorf("%w: path contained invalid characters that might allow command execution", tools.ErrArguments)
		}

		relPath, err := fs.GetRelativePath(g.ProjectPath, *path)
		if err != nil {
			return "", fmt.Errorf("%w: path error: %w", tools.ErrArguments, err)
		}

		fullPath = relPath
	}

	params := []string{
		"diff",
		"--no-color",
	}

	if stat := args.GetBool(ParamStat); stat != nil && *stat {
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
