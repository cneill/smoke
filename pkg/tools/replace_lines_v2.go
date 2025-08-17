package tools

import (
	"bytes"
	"context"
	"fmt"
	"os"

	"github.com/cneill/smoke/pkg/utils"
)

const (
	ReplaceLinesV2Path      = "path"
	ReplaceLinesV2StartLine = "start_line"
	ReplaceLinesV2EndLine   = "end_line"
	ReplaceLinesV2Replace   = "replace"
)

type ReplaceLinesV2Tool struct {
	ProjectPath string
}

func (r *ReplaceLinesV2Tool) Name() string { return ToolReplaceLines }

func (r *ReplaceLinesV2Tool) Description() string {
	return fmt.Sprintf(
		"Replace the content between lines %q and %q in the file specified in %q with the contents in %q. Line "+
			"numbers are 1-indexed, as they are in the output values of `read_file`, `grep`, etc. and should match "+
			"those values.\n\n"+
			`Examples:
			
%s=1, %s=1, %s="", text="a\nb\nc\n" => "b\nc\n"
%s=1, %s=2, %s="x\ny\n", text="a\nb\nc\n" => "x\ny\nc\n"`,
		ReplaceLinesV2StartLine, ReplaceLinesV2EndLine, ReplaceLinesV2Path, ReplaceLinesV2Replace,
		ReplaceLinesV2StartLine, ReplaceLinesV2EndLine, ReplaceLinesV2Replace,
		ReplaceLinesV2StartLine, ReplaceLinesV2EndLine, ReplaceLinesV2Replace,
	)
}

func (r *ReplaceLinesV2Tool) Params() Params {
	return Params{
		{
			Key:         ReplaceLinesV2Path,
			Description: "The path of the file where lines will be replaced",
			Type:        ParamTypeString,
			Required:    true,
		},
		{
			Key:         ReplaceLinesV2StartLine,
			Description: fmt.Sprintf("The first line to replace with the text in %q", ReplaceLinesV2Replace),
			Type:        ParamTypeNumber,
			Required:    true,
		},
		{
			Key:         ReplaceLinesV2EndLine,
			Description: fmt.Sprintf("The last line to replace with the text in %q", ReplaceLinesV2Replace),
			Type:        ParamTypeNumber,
			Required:    true,
		},
		{
			Key: ReplaceLinesV2Replace,
			Description: fmt.Sprintf(
				"The string content that will replace the lines specified by the line numbers in %q and %q",
				ReplaceLinesV2StartLine, ReplaceLinesV2EndLine,
			),
			Type:     ParamTypeString,
			Required: true,
		},
	}
}

func (r *ReplaceLinesV2Tool) Run(_ context.Context, args Args) (string, error) {
	path := args.GetString(ReplaceLinesPath)
	if path == nil {
		return "", fmt.Errorf("%w: no path supplied", ErrArguments)
	}

	fullPath, err := utils.GetRelativePath(r.ProjectPath, *path)
	if err != nil {
		return "", fmt.Errorf("%w: path error: %w", ErrArguments, err)
	}

	startLine := args.GetInt(ReplaceLinesV2StartLine)
	endLine := args.GetInt(ReplaceLinesV2EndLine)
	replace := args.GetString(ReplaceLinesV2Replace)

	switch {
	case startLine == nil || endLine == nil || replace == nil:
		return "", fmt.Errorf(
			"%w: missing %q, %q, or %q",
			ErrArguments, ReplaceLinesV2StartLine, ReplaceLinesV2EndLine, ReplaceLinesV2Replace,
		)
	case *startLine < 1 || *endLine < 1:
		return "", fmt.Errorf("%w: %q or %q is less than 1", ErrArguments, ReplaceLinesV2StartLine, ReplaceLinesV2EndLine)
	case *startLine > *endLine:
		return "", fmt.Errorf("%w: %q is greater than %q", ErrArguments, ReplaceLinesV2StartLine, ReplaceLinesV2EndLine)
	}

	data, err := os.ReadFile(fullPath)
	if err != nil {
		return "", fmt.Errorf("%w: failed to read file %q: %w", ErrFileSystem, *path, err)
	}

	lines := bytes.Split(data, []byte("\n"))

	if *endLine > len(lines) {
		return "", fmt.Errorf("%w: %q is beyond the end of the file", ErrArguments, ReplaceLinesV2EndLine)
	}

	buf := &bytes.Buffer{}
	if *startLine > 1 {
		if _, err := buf.Write(bytes.Join(lines[0:*startLine-1], []byte("\n"))); err != nil {
			return "", fmt.Errorf("failed to write leading lines to buffer: %w", err)
		}

		buf.WriteRune('\n')
	}

	if _, err := buf.WriteString(*replace); err != nil {
		return "", fmt.Errorf("failed to write replace to buffer: %w", err)
	}

	if *endLine < len(lines) {
		if _, err := buf.Write(bytes.Join(lines[*endLine:], []byte("\n"))); err != nil {
			return "", fmt.Errorf("failed to write trailing lines to buffer: %w", err)
		}
	}

	data = buf.Bytes()

	if err := os.WriteFile(fullPath, data, 0o644); err != nil {
		return "", fmt.Errorf("%w: failed to write contents to %q: %w", ErrFileSystem, fullPath, err)
	}

	newLines := bytes.Split(data, []byte("\n"))

	return fmt.Sprintf("Replaced requested lines in %q.\n%s\nNew content:\n%s", *path, LineSep, utils.WithLineNumbers(newLines)), nil
}
