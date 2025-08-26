package tools

import (
	"bytes"
	"context"
	"fmt"
	"os"

	"github.com/cneill/smoke/pkg/fs"
	"github.com/cneill/smoke/pkg/utils"
)

const (
	ReadFilePath      = "path"
	ReadFileStartLine = "start_line"
	ReadFileEndLine   = "end_line"
)

type ReadFileTool struct {
	ProjectPath string
}

var _ = Tool(&ReadFileTool{})

func NewReadFileTool(projectPath, _ string) Tool {
	return &ReadFileTool{ProjectPath: projectPath}
}

func (r *ReadFileTool) Name() string { return ToolReadFile }
func (r *ReadFileTool) Description() string {
	return fmt.Sprintf(
		"Read the contents of a file. If you just want to read the whole file, don't include %q/%q",
		ReadFileStartLine,
		ReadFileEndLine,
	)
}

func (r *ReadFileTool) Params() Params {
	return Params{
		{
			Key:      ReadFilePath,
			Type:     ParamTypeString,
			Required: true,
		},
		{
			Key:         ReadFileStartLine,
			Description: "The starting line number to read (1 by default)",
			Type:        ParamTypeNumber,
			Required:    false,
		},
		{
			Key: ReadFileEndLine,
			Description: "The last line number to read (end of file by default). Do not provide a huge value here, " +
				"just leave empty if you want to read the whole file.",
			Type:     ParamTypeNumber,
			Required: false,
		},
	}
}

func (r *ReadFileTool) Run(_ context.Context, args Args) (string, error) { //nolint:cyclop
	path := args.GetString(ReadFilePath)
	if path == nil {
		return "", fmt.Errorf("%w: no path supplied", ErrArguments)
	}

	fullPath, err := fs.GetRelativePath(r.ProjectPath, *path)
	if err != nil {
		return "", fmt.Errorf("%w: path error: %w", ErrArguments, err)
	}

	var (
		start int64 = 1
		end   int64 = -1
	)

	if startArg := args.GetInt64(ReadFileStartLine); startArg != nil {
		if *startArg < 1 {
			return "", fmt.Errorf("%w: %q must be >= 1", ErrArguments, ReadFileStartLine)
		}

		start = *startArg
	}

	if endArg := args.GetInt64(ReadFileEndLine); endArg != nil {
		if *endArg < 1 {
			return "", fmt.Errorf("%w: %q must be >= 1", ErrArguments, ReadFileEndLine)
		}

		end = *endArg
	}

	if end != -1 && start > end {
		return "", fmt.Errorf("%w: %q must be <= %q", ErrArguments, ReadFileStartLine, ReadFileEndLine)
	}

	contents, err := os.ReadFile(fullPath)
	if err != nil {
		return "", fmt.Errorf("%w: failed to read file %q: %w", ErrFileSystem, fullPath, err)
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
		return "", fmt.Errorf("%w: %q is beyond the end of the file", ErrArguments, ReadFileStartLine)
	}

	output := utils.WithLineNumbers(lines[start-1:end], int(start))

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
