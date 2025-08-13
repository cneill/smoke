package tools

import (
	"bytes"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/cneill/smoke/pkg/utils"
)

const (
	GoTestPath = "path"
)

type GoTestTool struct {
	ProjectPath string
}

func (g *GoTestTool) Name() string { return "go_test" }

func (g *GoTestTool) Description() string {
	return fmt.Sprintf(
		"Runs `go test` against the file/directory specified in %q, or the whole project if not specified. "+
			"If a file is provided, tests in its containing directory will be run. Output is the raw `go test -json` "+
			"stream.",
		GoTestPath,
	)
}

func (g *GoTestTool) Params() Params {
	return Params{
		{
			Key:         GoTestPath,
			Description: "The path of the directory/file to test",
			Type:        ParamTypeString,
			Required:    false,
		},
	}
}

func (g *GoTestTool) Run(args Args) (string, error) {
	targetPath := g.ProjectPath

	if _, err := exec.LookPath("go"); err != nil {
		slog.Error("go not found on the system", "error", err)
		return "", fmt.Errorf("%w: go not found on the system", ErrFileSystem)
	}

	// path is optional
	if path := args.GetString(GoTestPath); path != nil {
		relPath, err := utils.GetRelativePath(g.ProjectPath, *path)
		if err != nil {
			return "", fmt.Errorf("%w: path error: %w", ErrArguments, err)
		}

		targetPath = relPath
	}

	stat, err := os.Stat(targetPath)
	if err != nil {
		return "", fmt.Errorf("%w: failed to stat path %q: %w", ErrFileSystem, targetPath, err)
	}

	var targetDir string
	if stat.IsDir() {
		targetDir = targetPath
	} else {
		targetDir = filepath.Dir(targetPath)
	}

	targetDir += "/..."

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	cmd := exec.Command("go", "test", "-json", targetDir)
	cmd.Dir = g.ProjectPath
	cmd.Stdout = stdout
	cmd.Stderr = stderr

	if err := cmd.Run(); err != nil {
		// Treat failing tests as a non-fatal condition: return whatever JSON we captured.
		slog.Warn("go test exited with error", "target_dir", targetDir, "error", err, "stderr", stderr.String())
	}

	return stdout.String(), nil
}
