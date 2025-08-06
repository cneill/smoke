package tools

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/cneill/smack/pkg/paths"
)

const (
	ReplaceLinesPath    = "path"
	ReplaceLinesContent = "content"
	ReplaceLinesStart   = "start"
	ReplaceLinesEnd     = "end"
)

type ReplaceLinesTool struct {
	ProjectPath string
}

func (r *ReplaceLinesTool) Name() string { return "replace_lines" }

func (r *ReplaceLinesTool) Description() string {
	return "Replace specific lines in a file given a start and end line number."
}

func (r *ReplaceLinesTool) Params() Params {
	return Params{
		{
			Key:         ReplaceLinesPath,
			Description: "The path of the file where lines will be replaced.",
			Type:        ParamTypeString,
			Required:    true,
		},
		{
			Key:         ReplaceLinesContent,
			Description: "The new content to insert between the start and end lines.",
			Type:        ParamTypeString,
			Required:    true,
		},
		{
			Key:         ReplaceLinesStart,
			Description: "The starting line number for the replacement.",
			Type:        ParamTypeNumber,
			Required:    true,
		},
		{
			Key:         ReplaceLinesEnd,
			Description: "The ending line number for the replacement.",
			Type:        ParamTypeNumber,
			Required:    true,
		},
	}
}

func (r *ReplaceLinesTool) Run(args Args) (string, error) {
	path := args.GetString(ReplaceLinesPath)
	if path == nil {
		return "", fmt.Errorf("no path supplied")
	}

	fullPath, err := paths.GetRelative(r.ProjectPath, *path)
	if err != nil {
		return "", fmt.Errorf("path error: %w", err)
	}

	content := args.GetString(ReplaceLinesContent)
	if content == nil {
		return "", fmt.Errorf("no content supplied")
	}

	start := args.GetInt64(ReplaceLinesStart)
	end := args.GetInt64(ReplaceLinesEnd)

	if start == nil || end == nil {
		return "", fmt.Errorf("start or end line number not supplied")
	}

	if *start > *end {
		return "", errors.New("start line number cannot be greater than end line number")
	}

	data, err := os.ReadFile(fullPath)
	if err != nil {
		return "", fmt.Errorf("failed to read file %q: %w", fullPath, err)
	}

	lines := strings.Split(string(data), "\n")

	if int(*start) < 1 || int(*end) > len(lines) {
		return "", errors.New("line numbers out of range")
	}

	lines = append(lines[:*start-1], append(strings.Split(*content, "\n"), lines[*end-1:]...)...)

	if err := os.WriteFile(fullPath, []byte(strings.Join(lines, "\n")), 0o644); err != nil {
		return "", fmt.Errorf("failed to write contents to %q: %w", fullPath, err)
	}

	return fmt.Sprintf("Replaced lines %d to %d in %q", *start, *end, *path), nil
}
