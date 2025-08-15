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

func (r *ReplaceLinesTool) Name() string { return ToolReplaceLines }

func (r *ReplaceLinesTool) Description() string {
	return fmt.Sprintf(
		`Replace strings in the given file with new contents. Must supply EITHER %q AND %q parameters OR %q.
		This is an exact string replace, not a regular expression replace. You can use this for easy deletion.

		Examples:
		search="z", replace="c" on text "abz" will produce "abc"
		search="a\nb\n", replace="" on text "a\nb\nc\n" will produce "c\n"
		batches=["a", "x", "b", "y", "c", "z"] on text "abc" will produce "xyz"`,
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
			Key: ReplaceLinesSearch,
			Description: fmt.Sprintf(
				"The string content to search for that will be replaced by the contents of %q",
				ReplaceLinesReplace,
			),
			Type:     ParamTypeString,
			Required: false,
		},
		{
			Key: ReplaceLinesReplace,
			Description: fmt.Sprintf(
				"The string content that will replace all occurrences of the content in %q within the specified file",
				ReplaceLinesSearch,
			),
			Type:     ParamTypeString,
			Required: false,
		},
		{
			Key: ReplaceLinesBatch,
			Description: fmt.Sprintf(
				"Provide a batch of replacements in [<search>, <replace>, <search>, <replace>, ...] format. Mutually "+
					"exclusive with %q/%q parameters",
				ReplaceLinesSearch,
				ReplaceLinesReplace,
			),
			Type:     ParamTypeArray,
			ItemType: ParamTypeString,
			Required: false,
		},
	}
}

func (r *ReplaceLinesTool) Run(args Args) (string, error) {
	path := args.GetString(ReplaceLinesPath)
	if path == nil {
		return "", fmt.Errorf("%w: no path supplied", ErrArguments)
	}

	fullPath, err := utils.GetRelativePath(r.ProjectPath, *path)
	if err != nil {
		return "", fmt.Errorf("%w: path error: %w", ErrArguments, err)
	}

	searches, replaces, err := getSearchesReplaces(args)
	if err != nil {
		return "", fmt.Errorf("failed to interpret arguments: %w", err)
	}

	data, err := os.ReadFile(fullPath)
	if err != nil {
		return "", fmt.Errorf("%w: failed to read file %q: %w", ErrFileSystem, *path, err)
	}

	for i := range searches {
		data = bytes.ReplaceAll(data, []byte(searches[i]), []byte(replaces[i]))
	}

	if err := os.WriteFile(fullPath, data, 0o644); err != nil {
		return "", fmt.Errorf("%w: failed to write contents to %q: %w", ErrFileSystem, fullPath, err)
	}

	lines := bytes.Split(data, []byte("\n"))

	return fmt.Sprintf("Replaced requested lines in %q.\n%s\nNew content:\n%s", *path, LineSep, utils.WithLineNumbers(lines)), nil
}

// getSearchesReplaces returns 2 slices - one of "search" and one of "replace" - or an error.
func getSearchesReplaces(args Args) ([]string, []string, error) {
	var searches, replaces []string

	if search := args.GetString(ReplaceLinesSearch); search != nil && *search != "" {
		searches = []string{*search}
	}

	if replace := args.GetString(ReplaceLinesReplace); replace != nil {
		replaces = []string{*replace}
	}

	if len(searches) != len(replaces) {
		return nil, nil, fmt.Errorf("%w: must supply both '%s' and '%s' when one is specified", ErrArguments, ReplaceLinesSearch, ReplaceLinesReplace)
	} else if len(searches) > 0 && len(replaces) > 0 {
		if batch := args.GetStringSlice(ReplaceLinesBatch); batch != nil {
			return nil, nil, fmt.Errorf(
				"%w: must supply EITHER '%s' AND '%s' OR '%s'",
				ErrArguments,
				ReplaceLinesSearch,
				ReplaceLinesReplace,
				ReplaceLinesBatch,
			)
		}

		return searches, replaces, nil
	}

	return batchesToSearchesReplaces(args)
}

func batchesToSearchesReplaces(args Args) ([]string, []string, error) {
	var searches, replaces []string

	batches := args.GetStringSlice(ReplaceLinesBatch)
	if batches == nil {
		return nil, nil, fmt.Errorf(
			"%w: must supply EITHER '%s' AND '%s' OR '%s'",
			ErrArguments,
			ReplaceLinesSearch,
			ReplaceLinesReplace,
			ReplaceLinesBatch,
		)
	} else if len(batches)%2 != 0 {
		return nil, nil, fmt.Errorf(
			"%w: '%s' array must have even length, with items in the form [<search>, <replace>, <search>, <replace>, ...]",
			ErrArguments,
			ReplaceLinesBatch,
		)
	}

	searches, replaces = make([]string, len(batches)/2), make([]string, len(batches)/2)

	for batchIndex := 0; batchIndex < len(batches); batchIndex += 2 {
		if batches[batchIndex] == "" {
			return nil, nil, fmt.Errorf("%w: empty searches are not allowed in batch replaces", ErrArguments)
		}

		searches[batchIndex/2] = batches[batchIndex]
		replaces[batchIndex/2] = batches[batchIndex+1]
	}

	return searches, replaces, nil
}
