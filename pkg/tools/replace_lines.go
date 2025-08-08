package tools

import (
	"bytes"
	"fmt"
	"os"

	"github.com/cneill/smoke/pkg/utils"
)

const (
	ReplaceLinesPath   = "path"
	ReplaceLinesSearch = "search"
	// ReplaceLinesSearchRegex = "search_regex"
	ReplaceLinesReplace = "replace"
)

type ReplaceLinesTool struct {
	ProjectPath string
}

func (r *ReplaceLinesTool) Name() string { return "replace_lines" }

func (r *ReplaceLinesTool) Description() string {
	return "Replace strings in the given file with new contents."
}

func (r *ReplaceLinesTool) Params() Params {
	return Params{
		{
			Key:         ReplaceLinesPath,
			Description: "The path of the file where lines will be replaced",
			Type:        ParamTypeString,
			Required:    true,
		},
		{
			Key:         ReplaceLinesSearch,
			Description: fmt.Sprintf("The content to search for that will be replaced by the contents of '%s'", ReplaceLinesReplace),
			Type:        ParamTypeString,
			Required:    true,
		},
		{
			Key:         ReplaceLinesReplace,
			Description: fmt.Sprintf("The content that will replace all occurrences of the content in '%s' within the specified file", ReplaceLinesSearch),
			Type:        ParamTypeString,
			Required:    true,
		},
	}
}

func (r *ReplaceLinesTool) Run(args Args) (string, error) {
	path := args.GetString(ReplaceLinesPath)
	if path == nil {
		return "", fmt.Errorf("no path supplied")
	}

	fullPath, err := utils.GetRelativePath(r.ProjectPath, *path)
	if err != nil {
		return "", fmt.Errorf("path error: %w", err)
	}

	search := args.GetString(ReplaceLinesSearch)
	if search == nil || *search == "" {
		return "", fmt.Errorf("no search supplied")
	}

	replace := args.GetString(ReplaceLinesReplace)
	if replace == nil {
		return "", fmt.Errorf("no replace supplied")
	}

	data, err := os.ReadFile(fullPath)
	if err != nil {
		return "", fmt.Errorf("failed to read file %q: %w", *path, err)
	}

	newData := bytes.ReplaceAll(data, []byte(*search), []byte(*replace))

	if err := os.WriteFile(fullPath, newData, 0o644); err != nil {
		return "", fmt.Errorf("failed to write contents to %q: %w", fullPath, err)
	}

	return fmt.Sprintf("Replaced requested lines in %q", *path), nil
}
