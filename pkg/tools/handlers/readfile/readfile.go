package readfile

import (
	"bytes"
	"context"
	"fmt"
	"os"

	"github.com/cneill/smoke/pkg/fs"
	"github.com/cneill/smoke/pkg/tools"
	"github.com/cneill/smoke/pkg/tools/formatting"
)

const (
	Name = "read_file"

	ParamPath      = "path"
	ParamStartLine = "start_line"
	ParamEndLine   = "end_line"
)

type ReadFile struct {
	ProjectPath string
}

func New(projectPath, _ string) (tools.Tool, error) {
	return &ReadFile{ProjectPath: projectPath}, nil
}

func (r *ReadFile) Name() string { return Name }
func (r *ReadFile) Description() string {
	examples := tools.CollectExamples(r.Examples()...)

	return fmt.Sprintf("Read the contents of a file. If you just want to read the whole file, don't include %q/%q.%s",
		ParamStartLine, ParamEndLine, examples)
}

func (r *ReadFile) Examples() tools.Examples {
	return tools.Examples{
		{
			Description: `Read the whole "LICENSE" file in the root of the repository`,
			Args:        tools.Args{ParamPath: "LICENSE"},
		},
		{
			Description: `Read the first 20 lines of the "src/main.go" file`,
			Args: tools.Args{
				ParamPath:      "src/main.go",
				ParamStartLine: 1,
				ParamEndLine:   20,
			},
		},
		{
			Description: `Read from line 200 to the end of "data.log"`,
			Args: tools.Args{
				ParamPath:      "data.log",
				ParamStartLine: 200,
			},
		},
	}
}

func (r *ReadFile) Params() tools.Params {
	return tools.Params{
		{
			Key:         ParamPath,
			Description: "The path of the file to read",
			Type:        tools.ParamTypeString,
			Required:    true,
		},
		{
			Key:         ParamStartLine,
			Description: "The starting line number to read (1 by default)",
			Type:        tools.ParamTypeNumber,
			Required:    false,
		},
		{
			Key:         ParamEndLine,
			Description: "The last line number to read (end of file by default).",
			Type:        tools.ParamTypeNumber,
			Required:    false,
		},
	}
}

func (r *ReadFile) Run(_ context.Context, args tools.Args) (string, error) { //nolint:cyclop
	path := args.GetString(ParamPath)
	if path == nil {
		return "", fmt.Errorf("%w: no path supplied", tools.ErrArguments)
	}

	fullPath, err := fs.GetRelativePath(r.ProjectPath, *path)
	if err != nil {
		return "", fmt.Errorf("%w: path error: %w", tools.ErrArguments, err)
	}

	var (
		start int64 = 1
		end   int64 = -1
	)

	if startArg := args.GetInt64(ParamStartLine); startArg != nil {
		if *startArg < 1 {
			return "", fmt.Errorf("%w: %q must be >= 1", tools.ErrArguments, ParamStartLine)
		}

		start = *startArg
	}

	if endArg := args.GetInt64(ParamEndLine); endArg != nil {
		if *endArg < 1 {
			return "", fmt.Errorf("%w: %q must be >= 1", tools.ErrArguments, ParamEndLine)
		}

		end = *endArg
	}

	if end != -1 && start > end {
		return "", fmt.Errorf("%w: %q must be <= %q", tools.ErrArguments, ParamStartLine, ParamEndLine)
	}

	contents, err := os.ReadFile(fullPath)
	if err != nil {
		return "", fmt.Errorf("%w: failed to read file %q: %w", tools.ErrFileSystem, fullPath, err)
	}

	if isBinary(contents) {
		return "[binary content]", nil
	}

	lines := bytes.Split(contents, []byte("\n"))
	numLines := int64(len(lines))

	if end == -1 || end > numLines {
		end = numLines
	}

	if start > numLines {
		return "", fmt.Errorf("%w: %q is beyond the end of the file", tools.ErrArguments, ParamStartLine)
	}

	output := formatting.WithLineNumbers(lines[start-1:end], int(start))

	return string(output), nil
}

// isBinary checks if the given byte slice contains binary data by looking for null bytes and other non-printable
// characters.
func isBinary(data []byte) bool {
	checkSize := min(len(data), 8192)
	nullBytes := 0

	for idx := range checkSize {
		b := data[idx]
		// Count null bytes
		if b == 0 {
			nullBytes++
		}

		if nullBytes > 0 && idx > 0 && float64(nullBytes)/float64(idx) > 0.01 {
			return true
		}
	}

	return false
}
