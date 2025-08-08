package tools

import (
	"bytes"
	"fmt"
	"os"

	"github.com/cneill/smoke/pkg/utils"
)

const (
	ReplaceLinesPath    = "path"
	ReplaceLinesSearch  = "search"
	ReplaceLinesReplace = "replace"
	ReplaceLinesBatch   = "batch"
)

type ReplaceLinesTool struct {
	ProjectPath string
}

func (r *ReplaceLinesTool) Name() string { return "replace_lines" }

func (r *ReplaceLinesTool) Description() string {
	return fmt.Sprintf(
		"Replace strings in the given file with new contents. Must supply EITHER '%s' AND '%s' parameters OR '%s'.",
		ReplaceLinesSearch,
		ReplaceLinesReplace,
		ReplaceLinesBatch,
	)
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
			Required:    false,
		},
		{
			Key:         ReplaceLinesReplace,
			Description: fmt.Sprintf("The content that will replace all occurrences of the content in '%s' within the specified file", ReplaceLinesSearch),
			Type:        ParamTypeString,
			Required:    false,
		},
		{
			Key: ReplaceLinesBatch,
			Description: fmt.Sprintf(
				"Provide a batch of replacements in [<search>, <replace>, <search>, <replace>, ...] format. Mutually exclusive with '%s'/'%s' parameters.",
				ReplaceLinesSearch,
				ReplaceLinesReplace,
			),
			Type:     ParamTypeArray,
			Required: false,
		},
	}
}

func getSearchesReplaces(args Args) (searches, replaces []string, err error) {
	if search := args.GetString(ReplaceLinesSearch); search != nil && *search != "" {
		searches = []string{*search}
	}

	if replace := args.GetString(ReplaceLinesReplace); replace != nil && *replace != "" {
		replaces = []string{*replace}
	}

	if len(searches) != len(replaces) {
		return nil, nil, fmt.Errorf("must supply both '%s' and '%s' when one is specified", ReplaceLinesSearch, ReplaceLinesReplace)
	} else if len(searches) > 0 && len(replaces) > 0 {
		if batch := args.GetStringSlice(ReplaceLinesBatch); batch != nil {
			return nil, nil, fmt.Errorf("must supply EITHER '%s' AND '%s' OR '%s'", ReplaceLinesSearch, ReplaceLinesReplace, ReplaceLinesBatch)
		}

		return searches, replaces, nil
	}

	batches := args.GetStringSlice(ReplaceLinesBatch)
	if batches == nil {
		return nil, nil, fmt.Errorf("must supply EITHER '%s' AND '%s' OR '%s'", ReplaceLinesSearch, ReplaceLinesReplace, ReplaceLinesBatch)
	} else if len(batches)%2 != 0 {
		return nil, nil, fmt.Errorf(
			"'%s' array must have even length, with items in the form [<search>, <replace>, <search>, <replace>, ...]",
			ReplaceLinesBatch,
		)
	}

	searches, replaces = make([]string, len(batches)/2), make([]string, len(batches)/2)
	for i := 0; i < len(batches); i += 2 {
		searches[i] = batches[i]
		replaces[i] = batches[i+1]
	}

	return searches, replaces, nil
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

	// search := args.GetString(ReplaceLinesSearch)
	// if search == nil || *search == "" {
	// 	return "", fmt.Errorf("no search supplied")
	// }

	// replace := args.GetString(ReplaceLinesReplace)
	// if replace == nil {
	// 	return "", fmt.Errorf("no replace supplied")
	// }

	// batch := args.GetStringSlice(ReplaceLinesBatch)

	searches, replaces, err := getSearchesReplaces(args)
	if err != nil {
		return "", fmt.Errorf("failed to interpret parameters: %w", err)
	}

	data, err := os.ReadFile(fullPath)
	if err != nil {
		return "", fmt.Errorf("failed to read file %q: %w", *path, err)
	}

	for i := range searches {
		data = bytes.ReplaceAll(data, []byte(searches[i]), []byte(replaces[i]))
	}

	if err := os.WriteFile(fullPath, data, 0o644); err != nil {
		return "", fmt.Errorf("failed to write contents to %q: %w", fullPath, err)
	}

	return fmt.Sprintf("Replaced requested lines in %q", *path), nil
}
