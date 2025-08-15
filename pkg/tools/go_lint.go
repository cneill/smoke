package tools

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/cneill/smoke/pkg/utils"
)

const (
	GoLintPath = "path"
)

type GoLintTool struct {
	ProjectPath string
}

func (g *GoLintTool) Name() string { return ToolGoLint }

func (g *GoLintTool) Description() string {
	return fmt.Sprintf(
		"Runs the golangci-lint linter against the file/directory specified in %q, or the whole project directory if "+
			"not specified.",
		GoLintPath,
	)
}

func (g *GoLintTool) Params() Params {
	return Params{
		{
			Key:         GoLintPath,
			Description: "The path of the directory/file to lint",
			Type:        ParamTypeString,
			Required:    false,
		},
	}
}

type output struct {
	Issues []issue `json:"Issues"`
}

type issue struct {
	FromLinter           string   `json:"FromLinter"`
	Text                 string   `json:"Text"`
	Severity             string   `json:"Severity"`
	SourceLines          []string `json:"SourceLines"`
	Pos                  *pos     `json:"Pos"`
	ExpectNoLint         bool     `json:"ExpectNoLint"`
	ExpectedNoLintLinter string   `json:"ExpectedNoLintLinter"`
}

type pos struct {
	Filename string `json:"Filename"`
	Offset   int64  `json:"Offset"`
	Line     int64  `json:"Line"`
	Column   int64  `json:"Column"`
}

func (g *GoLintTool) Run(args Args) (string, error) { //nolint:cyclop,funlen
	targetPath := g.ProjectPath
	originalPath := g.ProjectPath

	if _, err := exec.LookPath("golangci-lint"); err != nil {
		slog.Error("golangci-lint not found on the system", "error", err)
		return "", fmt.Errorf("%w: golangci-lint not found on the system", ErrFileSystem)
	}

	// path is optional
	if path := args.GetString(GoLintPath); path != nil {
		relPath, err := utils.GetRelativePath(g.ProjectPath, *path)
		if err != nil {
			return "", fmt.Errorf("%w: path error: %w", ErrArguments, err)
		}

		targetPath = relPath
		originalPath = relPath
	}

	stat, err := os.Stat(targetPath)
	if err != nil {
		return "", fmt.Errorf("%w: failed to stat path %s: %w", ErrFileSystem, targetPath, err)
	}

	var targetFile string

	if !stat.IsDir() {
		targetPath, targetFile = filepath.Split(targetPath)
	}

	targetPath = filepath.Join(targetPath, "...")
	cmdArgs := []string{
		"run",
		"--out-format=json",
		"--issues-exit-code=0",
		"--show-stats=false",
		targetPath,
	}
	buf := &bytes.Buffer{}
	errBuf := &bytes.Buffer{}
	cmd := exec.Command("golangci-lint", cmdArgs...)
	cmd.Dir = g.ProjectPath
	cmd.Stdout = buf
	cmd.Stderr = errBuf

	if err := cmd.Run(); err != nil {
		stderr := errBuf.String()
		slog.Error("error from golangci-lint", "path", targetPath, "file", targetFile, "error", err, "stderr", stderr)

		return "", fmt.Errorf("%w: golangci-lint: %s", ErrCommandExecution, stderr)
	}

	results := output{}
	if err := json.Unmarshal(buf.Bytes(), &results); err != nil {
		slog.Error("error parsing golangci-lint output", "error", err)
		// TODO: Maybe revisit this..?
		return buf.String(), nil
	}

	targetIssues := []issue{}

	if targetFile == "" {
		targetIssues = results.Issues
	} else {
		for _, issue := range results.Issues {
			issuePath, err := utils.GetRelativePath(g.ProjectPath, issue.Pos.Filename)
			if err != nil {
				continue
			}

			// we can't always lint individual files successfully, so we lint its directory and pick out the relevant
			// issues
			if issuePath == originalPath {
				targetIssues = append(targetIssues, issue)
			}
		}
	}

	issues, err := json.Marshal(targetIssues)
	if err != nil {
		slog.Error("failed to render JSON issues", "error", err)
		// TODO: Maybe revisit this..?
		return buf.String(), nil
	}

	return string(issues), nil
}
