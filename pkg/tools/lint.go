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
	LintPath = "path"
)

type LintTool struct {
	ProjectPath string
}

func (l *LintTool) Name() string {
	return "lint"
}

func (l *LintTool) Description() string {
	return "Runs the golangci-lint linter against the specified file/directory, or the whole project directory if a " +
		"path is not specified."
}

func (l *LintTool) Params() Params {
	return Params{
		{
			Key:         LintPath,
			Description: "The path of the directory/file to lint",
			Type:        ParamTypeString,
			Required:    false,
		},
	}
}

type Output struct {
	Issues []Issue `json:"Issues"`
}

type Issue struct {
	FromLinter           string   `json:"FromLinter"`
	Text                 string   `json:"Text"`
	Severity             string   `json:"Severity"`
	SourceLines          []string `json:"SourceLines"`
	Pos                  *Pos     `json:"Pos"`
	ExpectNoLint         bool     `json:"ExpectNoLint"`
	ExpectedNoLintLinter string   `json:"ExpectedNoLintLinter"`
}

type Pos struct {
	Filename string `json:"Filename"`
	Offset   int64  `json:"Offset"`
	Line     int64  `json:"Line"`
	Column   int64  `json:"Column"`
}

func (l *LintTool) Run(args Args) (string, error) {
	fullPath := l.ProjectPath
	originalPath := l.ProjectPath

	if path := args.GetString(LintPath); path != nil {
		relPath, err := utils.GetRelativePath(l.ProjectPath, *path)
		if err != nil {
			return "", fmt.Errorf("path error: %w", err)
		}

		fullPath = relPath
		originalPath = relPath
	}

	stat, err := os.Stat(fullPath)
	if err != nil {
		return "", fmt.Errorf("failed to stat path %s: %w", fullPath, err)
	}

	var targetFile string

	if !stat.IsDir() {
		fullPath, targetFile = filepath.Split(fullPath)
	}

	fullPath = filepath.Join(fullPath, "...")
	cmdArgs := []string{
		"run",
		"--out-format=json",
		"--issues-exit-code=0",
		"--show-stats=false",
		fullPath,
	}

	buf := &bytes.Buffer{}
	errBuf := &bytes.Buffer{}
	cmd := exec.Command("golangci-lint", cmdArgs...)
	cmd.Dir = l.ProjectPath
	cmd.Stdout = buf
	cmd.Stderr = errBuf

	if err := cmd.Run(); err != nil {
		stderr := errBuf.String()
		slog.Error("error from golangci-lint execution", "path", fullPath, "file", targetFile, "error", err, "stderr", stderr)
		return "", fmt.Errorf("error from golangci-lint execution: %s", stderr)
	}

	results := Output{}
	if err := json.Unmarshal(buf.Bytes(), &results); err != nil {
		slog.Error("error parsing golangci-lint output", "err", err)
		// TODO: Maybe revisit this..?
		return buf.String(), nil
	}

	for _, issue := range results.Issues {
		slog.Debug("issue detected", "path", issue.Pos.Filename, "detail", issue.Text)
	}

	targetIssues := []Issue{}

	if targetFile == "" {
		targetIssues = results.Issues
	} else {
		for _, issue := range results.Issues {
			issuePath, err := utils.GetRelativePath(l.ProjectPath, issue.Pos.Filename)
			if err != nil {
				continue
			}

			if issuePath == originalPath {
				targetIssues = append(targetIssues, issue)
			}
		}
	}

	issues, err := json.Marshal(targetIssues)
	if err != nil {
		slog.Error("failed to render JSON issues", "err", err)
		// TODO: Maybe revisit this..?
		return buf.String(), nil
	}

	return string(issues), nil

	// output, err := cmd.CombinedOutput()
	// if err != nil {
	// 	slog.Warn("error from golangci-lint execution", "path", fullPath, "error", err)
	// }

	// return string(output), nil
}
