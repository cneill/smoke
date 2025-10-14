package gotest

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/cneill/smoke/pkg/fs"
	"github.com/cneill/smoke/pkg/tools"
)

const (
	Name      = "go_test"
	ParamPath = "path"
)

type GoTest struct {
	ProjectPath string
}

func New(projectPath, _ string) tools.Tool {
	return &GoTest{ProjectPath: projectPath}
}

func (g *GoTest) Name() string { return Name }
func (g *GoTest) Description() string {
	examples := tools.CollectExamples(g.Examples()...)

	return fmt.Sprintf("Runs `go test` against the file/directory specified in %q, or the whole project if not "+
		"specified. If a file is provided, tests in its containing directory will be run. Output is the raw `go test` "+
		"stream with coverage information.%s",
		ParamPath, examples,
	)
}

func (g *GoTest) Examples() tools.Examples {
	return tools.Examples{
		{
			Description: `Run "go test" on the "pkg/fs" directory`,
			Args:        tools.Args{ParamPath: "pkg/fs"},
		},
	}
}

func (g *GoTest) Params() tools.Params {
	return tools.Params{
		{
			Key:         ParamPath,
			Description: "The path of the directory/file to test",
			Type:        tools.ParamTypeString,
			Required:    false,
		},
	}
}

type GoTestResult struct {
	Action      string    `json:"Action"`
	Elapsed     float64   `json:"Elapsed"`
	FailedBuild string    `json:"FailedBuild"`
	ImportPath  string    `json:"ImportPath"`
	Output      string    `json:"Output"`
	Package     string    `json:"Package"`
	Test        string    `json:"Test"`
	Time        time.Time `json:"Time"`
}

func (g *GoTest) Run(ctx context.Context, args tools.Args) (string, error) {
	targetPath := g.ProjectPath

	if _, err := exec.LookPath("go"); err != nil {
		slog.Error("go not found on the system", "error", err)
		return "", fmt.Errorf("%w: go not found on the system", tools.ErrFileSystem)
	}

	// path is optional
	if path := args.GetString(ParamPath); path != nil {
		relPath, err := fs.GetRelativePath(g.ProjectPath, *path)
		if err != nil {
			return "", fmt.Errorf("%w: path error: %w", tools.ErrArguments, err)
		}

		targetPath = relPath
	}

	stat, err := os.Stat(targetPath)
	if err != nil {
		return "", fmt.Errorf("%w: failed to stat path %q: %w", tools.ErrFileSystem, targetPath, err)
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

	ctx, cancel := context.WithTimeout(ctx, time.Second*10)
	defer cancel()

	cmd := exec.CommandContext(ctx, "go", "test", "-p=1", "-parallel=1", "-cover", targetDir)
	cmd.Dir = g.ProjectPath
	cmd.Stdout = stdout
	cmd.Stderr = stderr

	if err := cmd.Run(); err != nil {
		// Treat failing tests as a non-fatal condition: return whatever JSON we captured.
		slog.Warn("go test exited with error", "target_dir", targetDir, "error", err, "stderr", stderr.String())
	}

	result := "stdout:\n" + stdout.String()
	if stderrString := stderr.String(); stderrString != "" {
		result += "\n\nstderr:\n" + stderr.String()
	}

	return result, nil
}
