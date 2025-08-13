package tools

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/cneill/smoke/pkg/utils"
)

const (
	ReadFilePath  = "path"
	ReadFileStart = "start"
	ReadFileEnd   = "end"
)

type ReadFileTool struct {
	ProjectPath string
}

var _ = Tool(&ReadFileTool{})

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

var _ = Tool(&ReadFileTool{})

func (r *ReadFileTool) Name() string { return ToolReadFile }
func (r *ReadFileTool) Description() string {
	return fmt.Sprintf(
		"Read the contents of a file. If you just want to read the whole file, don't include %q/%q",
		ReadFileStart,
		ReadFileEnd,
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
			Key:         ReadFileStart,
			Description: "The starting line number to read (1 by default)",
			Type:        ParamTypeNumber,
			Required:    false,
		},
		{
			Key:         ReadFileEnd,
			Description: "The last line number to read (end of file by default)",
			Type:        ParamTypeNumber,
			Required:    false,
		},
	}
}

func (r *ReadFileTool) Run(args Args) (string, error) { //nolint:cyclop,funlen
	path := args.GetString(ReadFilePath)
	if path == nil {
		return "", fmt.Errorf("%w: no path supplied", ErrArguments)
	}

	fullPath, err := utils.GetRelativePath(r.ProjectPath, *path)
	if err != nil {
		return "", fmt.Errorf("%w: path error: %w", ErrArguments, err)
	}

	var (
		start int64 = 1
		end   int64 = -1
	)

	if startArg := args.GetInt64(ReadFileStart); startArg != nil {
		if *startArg < 1 {
			return "", fmt.Errorf("%w: %q must be >= 1", ErrArguments, ReadFileStart)
		}

		start = *startArg
	}

	if endArg := args.GetInt64(ReadFileEnd); endArg != nil {
		if *endArg < 1 {
			return "", fmt.Errorf("%w: %q must be >= 1", ErrArguments, ReadFileEnd)
		}

		end = *endArg
	}

	if end != -1 && start > end {
		return "", fmt.Errorf("%w: %q must be <= %q", ErrArguments, ReadFileStart, ReadFileEnd)
	}

	contents, err := os.ReadFile(fullPath)
	if err != nil {
		return "", fmt.Errorf("%w: failed to read file %q: %w", ErrFileSystem, fullPath, err)
	}

	if isBinary(contents) {
		return "[binary content]", nil
	}

	lines := strings.Split(string(contents), "\n")
	width := len(strconv.Itoa(len(lines)))

	if start > int64(len(lines)) {
		return "", fmt.Errorf("%w: %q is beyond the end of the file", ErrArguments, ReadFileStart)
	} else if end > int64(len(lines)) {
		return "", fmt.Errorf("%w: %q is beyond the end of the file", ErrArguments, ReadFileEnd)
	}

	builder := &strings.Builder{}

	for lineNum, line := range lines {
		if int64(lineNum+1) < start {
			continue
		}

		if end > 1 && int64(lineNum+1) > end {
			break
		}

		fmt.Fprintf(builder, "%*d: %s\n", width, lineNum+1, line)
	}

	return builder.String(), nil
}
