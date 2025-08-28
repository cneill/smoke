package tools

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"time"

	"github.com/cneill/smoke/pkg/fs"
)

const (
	GoFumptPath = "path"
)

type GoFumptTool struct {
	ProjectPath string
}

func NewGoFumptTool(projectPath, _ string) Tool {
	return &GoFumptTool{ProjectPath: projectPath}
}

func (g *GoFumptTool) Name() string { return ToolGoFumpt }
func (g *GoFumptTool) Description() string {
	examples := CollectExamples(g.Examples()...)

	return fmt.Sprintf(
		"Runs the gofumpt formatter against the file/directory specified in %q, or the whole project directory if "+
			"not specified. Changes are written in place (-w) and the list of files formatted is returned (-l).%s",
		GoFumptPath, examples,
	)
}

func (g *GoFumptTool) Examples() Examples {
	return Examples{
		{
			Description: "Format the entire repository",
			Args:        Args{},
		},
		{
			Description: `Format the "pkg/tools" directory specifically`,
			Args:        Args{GoFumptPath: "pkg/tools"},
		},
	}
}

func (g *GoFumptTool) Params() Params {
	return Params{
		{
			Key:         GoFumptPath,
			Description: "The path of the directory/file to format",
			Type:        ParamTypeString,
			Required:    false,
		},
	}
}

func (g *GoFumptTool) Run(ctx context.Context, args Args) (string, error) {
	targetPath := g.ProjectPath

	if _, err := exec.LookPath("gofumpt"); err != nil {
		slog.Error("gofumpt not found on the system", "error", err)
		return "", fmt.Errorf("%w: gofumpt not found on the system", ErrFileSystem)
	}

	// path is optional
	if path := args.GetString(GoFumptPath); path != nil {
		relPath, err := fs.GetRelativePath(g.ProjectPath, *path)
		if err != nil {
			return "", fmt.Errorf("%w: path error: %w", ErrArguments, err)
		}

		targetPath = relPath
	}

	if _, err := os.Stat(targetPath); err != nil {
		return "", fmt.Errorf("%w: failed to stat path %q: %w", ErrFileSystem, targetPath, err)
	}

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}

	ctx, cancel := context.WithTimeout(ctx, time.Second*5)
	defer cancel()

	cmd := exec.CommandContext(ctx, "gofumpt", "-l", "-w", targetPath)
	cmd.Dir = g.ProjectPath
	cmd.Stdout = stdout
	cmd.Stderr = stderr

	if err := cmd.Run(); err != nil {
		slog.Error("error from gofumpt", "target_path", targetPath, "error", err, "stderr", stderr.String())
		return "", fmt.Errorf("%w: gofumpt: %s", ErrCommandExecution, stderr.String())
	}

	return stdout.String(), nil
}
