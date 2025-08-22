package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/cneill/smoke/pkg/fs"
)

const (
	GoLintPath = "path"
)

type GoLintTool struct {
	ProjectPath string
}

func NewGoLintTool(projectPath, _ string) Tool {
	return &GoLintTool{ProjectPath: projectPath}
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
	FromLinter           string          `json:"FromLinter"`
	Text                 string          `json:"Text"`
	Severity             string          `json:"Severity"`
	SourceLines          []string        `json:"SourceLines"`
	Pos                  *pos            `json:"Pos"`
	ExpectNoLint         bool            `json:"ExpectNoLint"`
	ExpectedNoLintLinter string          `json:"ExpectedNoLintLinter"`
	SuggestedFixes       []*suggestedFix `json:"SuggestedFixes"`
	LineRange            *lineRange      `json:"LineRange"`
}

type pos struct {
	Filename string `json:"Filename"`
	Offset   int64  `json:"Offset"`
	Line     int64  `json:"Line"`
	Column   int64  `json:"Column"`
}

type suggestedFix struct {
	Message   string      `json:"Message"`
	TextEdits []*textEdit `json:"TextEdits"`
}

type textEdit struct {
	Pos     int64  `json:"Pos"`
	End     int64  `json:"End"`
	NewText string `json:"NewText"`
}

type lineRange struct {
	From int64 `json:"From"`
	To   int64 `json:"To"`
}

func (g *GoLintTool) Run(ctx context.Context, args Args) (string, error) { //nolint:cyclop,funlen
	targetPath := g.ProjectPath
	originalPath := g.ProjectPath

	if _, err := exec.LookPath("golangci-lint"); err != nil {
		slog.Error("golangci-lint not found on the system", "error", err)
		return "", fmt.Errorf("%w: golangci-lint not found on the system", ErrFileSystem)
	}

	versionCtx, cancel := context.WithTimeout(ctx, time.Second*3)
	defer cancel()

	versionOutput, err := exec.CommandContext(versionCtx, "golangci-lint", "version").CombinedOutput()
	if err != nil {
		slog.Error("failed to get golangci-lint version", "error", err)
		return "", fmt.Errorf("%w: failed to get golangci-lint version: %w", ErrCommandExecution, err)
	} else if !bytes.Contains(versionOutput, []byte("has version 2")) {
		slog.Error("golangci-lint version <2", "output", string(versionOutput))
		return "", fmt.Errorf("%w: golangci-lint version <2", ErrCommandExecution)
	}

	// path is optional
	if path := args.GetString(GoLintPath); path != nil {
		relPath, err := fs.GetRelativePath(g.ProjectPath, *path)
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
		"--issues-exit-code=0",
		"--output.json.path=stdout",
		"--output.text.path=",
		"--output.tab.path=",
		"--output.html.path=",
		"--output.checkstyle.path=",
		"--output.code-climate.path=",
		"--output.junit-xml.path=",
		"--output.teamcity.path=",
		"--output.sarif.path=",
		"--max-same-issues=0",
		"--max-issues-per-linter=0",
		"--show-stats=false",
		targetPath,
	}
	// v1 args
	// cmdArgs := []string{
	// 	"run",
	// 	"--out-format=json",
	// 	"--issues-exit-code=0",
	// 	"--show-stats=false",
	// 	targetPath,
	// }
	buf := &bytes.Buffer{}
	errBuf := &bytes.Buffer{}

	lintCtx, lintCancel := context.WithTimeout(ctx, time.Second*60)
	defer lintCancel()

	cmd := exec.CommandContext(lintCtx, "golangci-lint", cmdArgs...)
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
			issuePath, err := fs.GetRelativePath(g.ProjectPath, issue.Pos.Filename)
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
