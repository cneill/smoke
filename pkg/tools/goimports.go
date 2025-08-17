package tools

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"time"

	"github.com/cneill/smoke/pkg/utils"
)

const (
	GoImportsPath = "path"
)

type GoImportsTool struct {
	ProjectPath string
}

func (g *GoImportsTool) Name() string { return ToolGoImports }

func (g *GoImportsTool) Description() string {
	return fmt.Sprintf(
		"Runs the goimports command to fix imports against the file/directory specified in %q, or the whole project "+
			"directory if not specified. Changes are written in place (-w) and the list of files formatted is "+
			"returned (-l).",
		GoImportsPath,
	)
}

func (g *GoImportsTool) Params() Params {
	return Params{
		{
			Key:         GoImportsPath,
			Description: "The path of the directory/file to format",
			Type:        ParamTypeString,
			Required:    false,
		},
	}
}

func (g *GoImportsTool) Run(args Args) (string, error) {
	targetPath := g.ProjectPath

	if _, err := exec.LookPath("goimports"); err != nil {
		slog.Error("goimports not found on the system", "error", err)
		return "", fmt.Errorf("%w: goimports not found on the system", ErrFileSystem)
	}

	// path is optional
	if path := args.GetString(GoImportsPath); path != nil {
		relPath, err := utils.GetRelativePath(g.ProjectPath, *path)
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

	ctx, cancel := context.WithTimeout(context.TODO(), time.Second*10)
	defer cancel()

	cmd := exec.CommandContext(ctx, "goimports", "-l", "-w", targetPath)
	cmd.Dir = g.ProjectPath
	cmd.Stdout = stdout
	cmd.Stderr = stderr

	if err := cmd.Run(); err != nil {
		slog.Error("error from goimports", "target_path", targetPath, "error", err, "stderr", stderr.String())
		return "", fmt.Errorf("%w: goimports: %s", ErrCommandExecution, stderr.String())
	}

	return stdout.String(), nil
}
