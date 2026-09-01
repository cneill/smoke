package edit

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"

	smokefs "github.com/cneill/smoke/pkg/fs"
	"github.com/cneill/smoke/pkg/tools"
)

const (
	ParamPath    = "path"
	ParamEdits   = "edits"
	ParamOldText = "old_text"
	ParamNewText = "new_text"
)

type Edit struct {
	ProjectPath string
}

type replacement struct {
	start   int
	end     int
	newText string
}

func New(projectPath, _ string) (tools.Tool, error) {
	return &Edit{ProjectPath: projectPath}, nil
}

func (e *Edit) Name() string { return tools.NameEdit }

func (e *Edit) Description() string {
	examples := tools.CollectExamples(e.Examples()...)
	description := "Apply one or more string replacements to the file at %q. Provide an %q array of objects, " +
		"each with %q and %q. Every %q must match EXACTLY once in the original file contents, otherwise error. All " +
		"matches are resolved against the ORIGINAL file. Overlapping replacements are rejected. Don't send multiple " +
		"NEARBY edits, merge them into one edit. Keep edits as small as possible, avoid useless filler.%s"

	return fmt.Sprintf(
		description,
		ParamPath,
		ParamEdits,
		ParamOldText,
		ParamNewText,
		ParamOldText,
		examples,
	)
}

func (e *Edit) Examples() tools.Examples {
	return tools.Examples{
		{
			Description: `Replace two distinct strings in "pkg/example.txt" based on the original file contents`,
			Args: tools.Args{
				ParamPath: "pkg/example.txt",
				ParamEdits: []any{
					map[string]any{
						ParamOldText: "alpha",
						ParamNewText: "beta",
					},
					map[string]any{
						ParamOldText: "gamma",
						ParamNewText: "delta",
					},
				},
			},
		},
	}
}

func (e *Edit) Params() tools.Params {
	return tools.Params{
		{
			Key:         ParamPath,
			Description: "The path of the file to edit",
			Type:        tools.ParamTypeString,
			Required:    true,
		},
		{
			Key:         ParamEdits,
			Description: "An array of string replacement operations to apply against the original file contents",
			Type:        tools.ParamTypeArray,
			ItemType:    tools.ParamTypeObject,
			Required:    true,
			NestedParams: tools.Params{
				{
					Key:         ParamOldText,
					Description: "The exact original string that must appear exactly once in the file",
					Type:        tools.ParamTypeString,
					Required:    true,
				},
				{
					Key:         ParamNewText,
					Description: "The replacement string to splice in for the matching original string",
					Type:        tools.ParamTypeString,
					Required:    true,
				},
			},
		},
	}
}

func (e *Edit) Run(_ context.Context, args tools.Args) (*tools.Output, error) {
	path := args.GetString(ParamPath)
	if path == nil {
		return nil, fmt.Errorf("%w: no path supplied", tools.ErrArguments)
	}

	fullPath, err := smokefs.GetRelativePath(e.ProjectPath, *path)
	if err != nil {
		return nil, fmt.Errorf("%w: path error: %w", tools.ErrArguments, err)
	}

	edits := args.GetArgsObjectSlice(ParamEdits)
	if len(edits) == 0 {
		return nil, fmt.Errorf("%w: no edits supplied", tools.ErrArguments)
	}

	original, err := os.ReadFile(fullPath)
	if err != nil {
		return nil, fmt.Errorf("%w: failed to read file %q: %w", tools.ErrFileSystem, *path, err)
	}

	replacements, err := e.collectReplacements(string(original), edits)
	if err != nil {
		return nil, err
	}

	updated := applyReplacements(string(original), replacements)
	if err := os.WriteFile(fullPath, []byte(updated), 0o644); err != nil { //nolint:gosec
		return nil, fmt.Errorf("%w: failed to write contents to %q: %w", tools.ErrFileSystem, *path, err)
	}

	return &tools.Output{Text: fmt.Sprintf("Applied %d edit(s) to %q", len(replacements), *path)}, nil
}

func (e *Edit) collectReplacements(contents string, edits []tools.Args) ([]replacement, error) {
	replacements := make([]replacement, 0, len(edits))

	for idx, editArgs := range edits {
		oldText := editArgs.GetString(ParamOldText)

		newText := editArgs.GetString(ParamNewText)
		if oldText == nil || newText == nil {
			return nil, fmt.Errorf("%w: %s[%d] missing %q or %q", tools.ErrArguments, ParamEdits, idx, ParamOldText, ParamNewText)
		}

		start, err := matchIdx(contents, *oldText)
		if err != nil {
			return nil, fmt.Errorf("%w: %s[%d]: %w", tools.ErrArguments, ParamEdits, idx, err)
		}

		replacements = append(replacements, replacement{
			start:   start,
			end:     start + len(*oldText),
			newText: *newText,
		})
	}

	sort.Slice(replacements, func(i, j int) bool {
		return replacements[i].start < replacements[j].start
	})

	for i := 1; i < len(replacements); i++ {
		if replacements[i].start < replacements[i-1].end {
			return nil, fmt.Errorf("%w: %s[%d] overlaps %s[%d]", tools.ErrArguments, ParamEdits, i-1, ParamEdits, i)
		}
	}

	return replacements, nil
}

func matchIdx(contents, oldText string) (int, error) {
	matches := []int{}

	if oldText == "" {
		return -1, fmt.Errorf("%q must not be empty", ParamOldText)
	}

	startIdx := 0

	for startIdx < len(contents) {
		nextMatch := strings.Index(contents[startIdx:], oldText)
		if nextMatch == -1 {
			if len(matches) == 0 {
				return -1, fmt.Errorf("%q pattern not found in original file contents", ParamOldText)
			}

			break
		}

		matches = append(matches, startIdx+nextMatch)
		startIdx += nextMatch + 1
	}

	if len(matches) == 1 {
		return matches[0], nil
	}

	currentMatch := 0
	lineCount := 1
	matchLines := make([]string, len(matches))

	// TODO: handle utf-8 runes?
	for i, char := range contents {
		if currentMatch >= len(matches) {
			break
		}

		if i == matches[currentMatch] {
			matchLines[currentMatch] = strconv.FormatInt(int64(lineCount), 10)
			currentMatch++
		}

		if char == '\n' {
			lineCount++
			continue
		}
	}

	return -1, fmt.Errorf("multiple matches (lines %s)", strings.Join(matchLines, ", "))
}

func applyReplacements(contents string, replacements []replacement) string {
	if len(replacements) == 0 {
		return contents
	}

	var builder strings.Builder

	last := 0
	for _, repl := range replacements {
		builder.WriteString(contents[last:repl.start])
		builder.WriteString(repl.newText)
		last = repl.end
	}

	builder.WriteString(contents[last:])

	return builder.String()
}
