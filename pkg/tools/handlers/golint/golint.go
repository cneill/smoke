package golint

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
	"github.com/cneill/smoke/pkg/tools"
)

const (
	ParamPath = "path"
)

type GoLint struct {
	ProjectPath string
}

func New(projectPath, _ string) (tools.Tool, error) {
	if _, err := exec.LookPath("golangci-lint"); err != nil {
		return nil, fmt.Errorf("%w: golangci-lint not found on the system", tools.ErrMissingExecutable)
	}

	versionCtx, cancel := context.WithTimeout(context.Background(), time.Second*3)
	defer cancel()

	versionOutput, err := exec.CommandContext(versionCtx, "golangci-lint", "version").CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("%w: failed to get golangci-lint version: %w", tools.ErrCommandExecution, err)
	} else if !bytes.Contains(versionOutput, []byte("has version 2")) {
		return nil, fmt.Errorf("%w: golangci-lint version <2", tools.ErrCommandExecution)
	}

	return &GoLint{ProjectPath: projectPath}, nil
}

func (g *GoLint) Name() string { return tools.NameGoLint }
func (g *GoLint) Description() string {
	examples := tools.CollectExamples(g.Examples()...)

	return fmt.Sprintf("Runs the golangci-lint linter against the file/directory specified in %q, or the whole "+
		"project directory if not specified.%s",
		ParamPath, examples,
	)
}

func (g *GoLint) Examples() tools.Examples {
	return tools.Examples{
		{
			Description: `Lint the "pkg/llms" directory`,
			Args:        tools.Args{ParamPath: "pkg/llms"},
		},
	}
}

func (g *GoLint) Params() tools.Params {
	return tools.Params{
		{
			Key:         ParamPath,
			Description: "The path of the directory/file to lint",
			Type:        tools.ParamTypeString,
			Required:    false,
		},
	}
}

type golintOutput struct {
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

func (g *GoLint) Run(ctx context.Context, args tools.Args) (*tools.Output, error) { //nolint:cyclop,funlen
	targetPath := g.ProjectPath
	originalPath := g.ProjectPath

	// path is optional
	if path := args.GetString(ParamPath); path != nil {
		relPath, err := fs.GetRelativePath(g.ProjectPath, *path)
		if err != nil {
			return nil, fmt.Errorf("%w: path error: %w", tools.ErrArguments, err)
		}

		targetPath = relPath
		originalPath = relPath
	}

	stat, err := os.Stat(targetPath)
	if err != nil {
		return nil, fmt.Errorf("%w: failed to stat path %s: %w", tools.ErrFileSystem, targetPath, err)
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

		return nil, fmt.Errorf("%w: golangci-lint: %s", tools.ErrCommandExecution, stderr)
	}

	output := &tools.Output{Text: buf.String()}

	results := golintOutput{}
	if err := json.Unmarshal(buf.Bytes(), &results); err != nil {
		slog.Error("error parsing golangci-lint output", "error", err)
		// TODO: Maybe revisit this..?
		return output, nil
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
		return output, nil
	}

	output.Text = string(issues)

	return output, nil
}
