package tools

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

func (g *GoTestTool) Run(ctx context.Context, args Args) (string, error) {
	targetPath := g.ProjectPath

	if _, err := exec.LookPath("go"); err != nil {
		slog.Error("go not found on the system", "error", err)
		return "", fmt.Errorf("%w: go not found on the system", ErrFileSystem)
	}

	// path is optional
	if path := args.GetString(GoTestPath); path != nil {
		relPath, err := fs.GetRelativePath(g.ProjectPath, *path)
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

	// TODO: revisit JSON and stitching together its results - forces the -v flag and produces tons of junk
	// cmd := exec.Command("go", "test", "-json", targetDir)
	// results := []GoTestResult{}
	//
	// scanner := bufio.NewScanner(stdout)
	// for scanner.Scan() {
	// 	result := GoTestResult{}
	// 	if err := json.Unmarshal(scanner.Bytes(), &result); err != nil {
	// 		slog.Warn("failed to read JSON line", "error", err, "line", scanner.Text())
	// 		break
	// 	}
	//
	// 	results = append(results, result)
	// }
	//
	// if err := scanner.Err(); err != nil {
	// 	slog.Warn("failed to read JSON output from go test", "error", err)
	// 	return stdout.String(), nil
	// }
	//
	// usefulResults := make([]GoTestResult, 0, len(results))
	//
	// for _, result := range results {
	// 	if result.Action != "fail" {
	// 		continue
	// 	}
	//
	// 	usefulResults = append(usefulResults, result)
	// }
	//
	// usefulBytes, err := json.Marshal(usefulResults)
	// if err != nil {
	// 	slog.Warn("failed to marshal JSON results from go test", "error", err)
	// 	return stdout.String(), nil
	// }

	// return string(usefulBytes), nil
	return stdout.String(), nil
}
