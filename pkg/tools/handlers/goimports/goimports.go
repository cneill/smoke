package goimports

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
	Name      = "goimports"
	ParamPath = "path"
)

type GoImports struct {
	ProjectPath string
}

func New(projectPath, _ string) tools.Tool {
	// TODO: allow error-return here, return / log error if binary not found, don't crash everything
	return &GoImports{ProjectPath: projectPath}
}

func (g *GoImports) Name() string { return Name }
func (g *GoImports) Description() string {
	examples := tools.CollectExamples(g.Examples()...)

	return fmt.Sprintf("Runs the goimports command to fix imports against the file/directory specified in %q, or the "+
		"whole project directory if not specified. Changes are written in place (-w) and the list of files formatted "+
		"is returned (-l).%s",
		ParamPath, examples,
	)
}

func (g *GoImports) Examples() tools.Examples {
	return tools.Examples{
		{
			Description: "Run goimports on the entire repository",
			Args:        tools.Args{},
		},
		{
			Description: `Run goimports on the "pkg/tools/file.go" file`,
			Args:        tools.Args{ParamPath: "pkg/tools/file.go"},
		},
	}
}

func (g *GoImports) Params() tools.Params {
	return tools.Params{
		{
			Key:         ParamPath,
			Description: "The path of the directory/file to format",
			Type:        tools.ParamTypeString,
			Required:    false,
		},
	}
}

func (g *GoImports) Run(ctx context.Context, args tools.Args) (string, error) {
	targetPath := g.ProjectPath

	if _, err := exec.LookPath("goimports"); err != nil {
		slog.Error("goimports not found on the system", "error", err)
		return "", fmt.Errorf("%w: goimports not found on the system", tools.ErrFileSystem)
	}

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

	ctx, cancel := context.WithTimeout(ctx, time.Second*10)
	defer cancel()

	cmd := exec.CommandContext(ctx, "goimports", "-l", "-w", targetPath)
	cmd.Dir = g.ProjectPath
	cmd.Stdout = stdout
	cmd.Stderr = stderr

	if err := cmd.Run(); err != nil {
		slog.Error("error from goimports", "target_path", targetPath, "error", err, "stderr", stderr.String())
		return "", fmt.Errorf("%w: goimports: %s", tools.ErrCommandExecution, stderr.String())
	}

	return stdout.String(), nil
}
