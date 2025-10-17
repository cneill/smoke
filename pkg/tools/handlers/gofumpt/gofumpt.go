package gofumpt

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"time"

	"github.com/cneill/smoke/pkg/fs"
	"github.com/cneill/smoke/pkg/tools"
)

const (
	ParamPath = "path"
)

type GoFumpt struct {
	ProjectPath string
}

func New(projectPath, _ string) (tools.Tool, error) {
	if _, err := exec.LookPath("gofumpt"); err != nil {
		return nil, fmt.Errorf("%w: gofumpt not found on the system", tools.ErrMissingExecutable)
	}

	return &GoFumpt{ProjectPath: projectPath}, nil
}

func (g *GoFumpt) Name() string { return tools.NameGoFumpt }
func (g *GoFumpt) Description() string {
	examples := tools.CollectExamples(g.Examples()...)

	return fmt.Sprintf(
		"Runs the gofumpt formatter against the file/directory specified in %q, or the whole project directory if "+
			"not specified. Changes are written in place (-w) and the list of files formatted is returned (-l).%s",
		ParamPath, examples,
	)
}

func (g *GoFumpt) Examples() tools.Examples {
	return tools.Examples{
		{
			Description: "Format the entire repository",
			Args:        tools.Args{},
		},
		{
			Description: `Format the "pkg/tools" directory specifically`,
			Args:        tools.Args{ParamPath: "pkg/tools"},
		},
	}
}

func (g *GoFumpt) Params() tools.Params {
	return tools.Params{
		{
			Key:         ParamPath,
			Description: "The path of the directory/file to format",
			Type:        tools.ParamTypeString,
			Required:    false,
		},
	}
}

func (g *GoFumpt) Run(ctx context.Context, args tools.Args) (string, error) {
	targetPath := g.ProjectPath

	// path is optional
	if path := args.GetString(ParamPath); path != nil {
		relPath, err := fs.GetRelativePath(g.ProjectPath, *path)
		if err != nil {
			return "", fmt.Errorf("%w: path error: %w", tools.ErrArguments, err)
		}

		targetPath = relPath
	}

	if _, err := os.Stat(targetPath); err != nil {
		return "", fmt.Errorf("%w: failed to stat path %q: %w", tools.ErrFileSystem, targetPath, err)
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
		return "", fmt.Errorf("%w: gofumpt: %s", tools.ErrCommandExecution, stderr.String())
	}

	return stdout.String(), nil
}
