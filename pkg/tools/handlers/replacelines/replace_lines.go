package replacelines

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/cneill/smoke/pkg/fs"
	"github.com/cneill/smoke/pkg/tools"
	"github.com/cneill/smoke/pkg/tools/formatting"
)

const (
	Name = "replace_lines"

	ParamPath      = "path"
	ParamStartLine = "start_line"
	ParamEndLine   = "end_line"
	ParamReplace   = "replace"
)

type ReplaceLines struct {
	ProjectPath string
}

func New(projectPath, _ string) (tools.Tool, error) {
	return &ReplaceLines{ProjectPath: projectPath}, nil
}

func (r *ReplaceLines) Name() string { return Name }
func (r *ReplaceLines) Description() string {
	examples := tools.CollectExamples(r.Examples()...)

	return fmt.Sprintf(
		"Replace the content between lines %q and %q in the file specified in %q with the contents in %q. Line "+
			"numbers are 1-indexed, as they are in the output values of %q, %q, etc. and should match those values. "+
			"If you want to edit an empty file, use %q=0 and %q=0. If you want to replace only some content in a "+
			"series of lines, be sure to include the old lines' content in %q where necessary, preserving spacing, "+
			"parentheses, curly braces, etc.%s",
		ParamStartLine, ParamEndLine, ParamPath, ParamReplace,
		tools.ToolGrep, tools.ToolReadFile,
		ParamStartLine, ParamEndLine,
		ParamReplace, examples,
	)
}

func (r *ReplaceLines) Examples() tools.Examples {
	return tools.Examples{
		{
			Description: `Add "hello, world" to the top of the file "empty_file.txt" in the project directory`,
			Args: tools.Args{
				ParamPath:      "empty_file.txt",
				ParamStartLine: 0,
				ParamEndLine:   0,
				ParamReplace:   "hello, world",
			},
		},
		{
			Description: `Delete the first line of "existing_file.txt", turning e.g. "a\nb\nc\n" into "b\nc\n"`,
			Args: tools.Args{
				ParamPath:      "existing_file.txt",
				ParamStartLine: 1,
				ParamEndLine:   1,
				ParamReplace:   "",
			},
		},
		{
			Description: `Replace the first 2 lines of "letters.txt", turning e.g. "a\nb\nc\n" into "x\ny\nc\n"`,
			Args: tools.Args{
				ParamPath:      "letters.txt",
				ParamStartLine: 1,
				ParamEndLine:   2,
				ParamReplace:   "x\ny\n",
			},
		},
	}
}

func (r *ReplaceLines) Params() tools.Params {
	return tools.Params{
		{
			Key:         ParamPath,
			Description: "The path of the file where lines will be replaced",
			Type:        tools.ParamTypeString,
			Required:    true,
		},
		{
			Key:         ParamStartLine,
			Description: fmt.Sprintf("The first line to replace with the text in %q", ParamReplace),
			Type:        tools.ParamTypeNumber,
			Required:    true,
		},
		{
			Key:         ParamEndLine,
			Description: fmt.Sprintf("The last line to replace with the text in %q", ParamReplace),
			Type:        tools.ParamTypeNumber,
			Required:    true,
		},
		{
			Key: ParamReplace,
			Description: fmt.Sprintf(
				"The string content that will replace the lines specified by the line numbers in %q and %q",
				ParamStartLine, ParamEndLine,
			),
			Type:     tools.ParamTypeString,
			Required: true,
		},
	}
}

func (r *ReplaceLines) Run(_ context.Context, args tools.Args) (string, error) {
	path := args.GetString(ParamPath)
	if path == nil {
		return "", fmt.Errorf("%w: no path supplied", tools.ErrArguments)
	}

	fullPath, err := fs.GetRelativePath(r.ProjectPath, *path)
	if err != nil {
		return "", fmt.Errorf("%w: path error: %w", tools.ErrArguments, err)
	}

	startLine := args.GetInt(ParamStartLine)
	endLine := args.GetInt(ParamEndLine)
	replace := args.GetString(ParamReplace)

	// validate that our args are reasonable
	switch {
	case startLine == nil || endLine == nil || replace == nil:
		return "", fmt.Errorf(
			"%w: missing %q, %q, or %q",
			tools.ErrArguments, ParamStartLine, ParamEndLine, ParamReplace,
		)
	case *startLine < 0 || *endLine < 0:
		return "", fmt.Errorf("%w: %q or %q is less than 0", tools.ErrArguments, ParamStartLine, ParamEndLine)
	case *startLine > *endLine:
		return "", fmt.Errorf("%w: %q is greater than %q", tools.ErrArguments, ParamStartLine, ParamEndLine)
	}

	data, err := os.ReadFile(fullPath)
	if err != nil {
		return "", fmt.Errorf("%w: failed to read file %q: %w", tools.ErrFileSystem, *path, err)
	}

	lines := bytes.Split(data, []byte("\n"))

	if *endLine > len(lines) {
		return "", fmt.Errorf("%w: %q is beyond the end of the file", tools.ErrArguments, ParamEndLine)
	}

	// write the lines before the replace, the contents of the replace, and the untouched lines after it
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

	// make sure we actually insert a LINE and not just text on another line
	if *replace != "" && !strings.HasSuffix(*replace, "\n") {
		buf.WriteRune('\n')
	}

	if *endLine < len(lines) {
		if _, err := buf.Write(bytes.Join(lines[*endLine:], []byte("\n"))); err != nil {
			return "", fmt.Errorf("failed to write trailing lines to buffer: %w", err)
		}
	}

	data = buf.Bytes()

	if err := os.WriteFile(fullPath, data, 0o644); err != nil {
		return "", fmt.Errorf("%w: failed to write contents to %q: %w", tools.ErrFileSystem, fullPath, err)
	}

	// Make sure we don't get a fake "line" when the file is now empty
	newLines := bytes.Split(data, []byte("\n"))
	if len(newLines) == 1 && len(newLines[0]) == 0 {
		newLines = [][]byte{}
	}

	// Generate contextual output instead of returning entire file
	contextOutput := r.generateContextOutput(*path, *startLine, *endLine, *replace, newLines)

	return contextOutput, nil
}

// calculateContextWindow determines how many lines of context to show before/after the replacement
func (r *ReplaceLines) calculateContextWindow(originalLinesReplaced, newLinesAdded int) int {
	totalChange := max(originalLinesReplaced, newLinesAdded)

	switch {
	case totalChange <= 3:
		return 5
	case totalChange <= 10:
		return 3
	default:
		return 2
	}
}

// generateContextOutput creates a focused context view around the replacement area
func (r *ReplaceLines) generateContextOutput(filePath string, startLine, endLine int, replacement string, newLines [][]byte) string {
	originalLinesReplaced := endLine - startLine + 1
	newLinesAdded := len(bytes.Split([]byte(replacement), []byte("\n")))

	if replacement != "" && !strings.HasSuffix(replacement, "\n") {
		newLinesAdded = len(strings.Split(replacement+"\n", "\n")) - 1
	}

	if replacement == "" {
		newLinesAdded = 0
	}

	contextLines := r.calculateContextWindow(originalLinesReplaced, newLinesAdded)

	// Calculate the new position where replacement content starts
	replacementStartLine := startLine
	replacementEndLine := startLine + newLinesAdded - 1

	if newLinesAdded == 0 {
		replacementEndLine = startLine - 1 // For deletions
	}

	// Calculate context window boundaries
	contextStart := max(1, replacementStartLine-contextLines)
	contextEnd := min(len(newLines), replacementEndLine+contextLines)

	if replacementEndLine < startLine {
		// For deletions, show context around where the deletion occurred
		contextEnd = min(len(newLines), startLine-1+contextLines)
	}

	// Handle edge case where entire file was replaced
	if startLine == 1 && endLine >= len(newLines) {
		// If we replaced the entire file and it's not too large, show it all
		if len(newLines) <= 50 {
			contextStart = 1
			contextEnd = len(newLines)
		} else {
			// For very large files, show first and last parts
			contextStart = 1
			contextEnd = min(25, len(newLines))
		}
	}

	// Extract context lines
	contextStartIdx := contextStart - 1
	contextEndIdx := contextEnd
	contextEndIdx = min(contextEndIdx, len(newLines))

	var contextLinesSlice [][]byte
	if contextStartIdx < contextEndIdx {
		contextLinesSlice = newLines[contextStartIdx:contextEndIdx]
	}

	contextOutput := formatting.WithLineNumbers(contextLinesSlice, contextStart)

	// Create summary message
	var summary string

	switch {
	case startLine == 0 && endLine == 0:
		summary = fmt.Sprintf("Added to top of file in %q.", filePath)
	case originalLinesReplaced == 1:
		if newLinesAdded == 0 {
			summary = fmt.Sprintf("Deleted line %d in %q.", startLine, filePath)
		} else {
			summary = fmt.Sprintf("Replaced line %d in %q.", startLine, filePath)
		}
	case newLinesAdded == 0:
		summary = fmt.Sprintf("Deleted lines %d-%d in %q.", startLine, endLine, filePath)
	default:
		summary = fmt.Sprintf("Replaced lines %d-%d in %q.", startLine, endLine, filePath)
	}

	if len(contextLinesSlice) == 0 {
		return summary + "\n" + tools.LineSep + "\n(File is now empty)"
	}

	contextLineNumbers := fmt.Sprintf("Context (lines %d-%d)", contextStart, contextEnd-1)
	if contextStart == contextEnd-1 {
		contextLineNumbers = fmt.Sprintf("Context (line %d)", contextStart)
	}

	return fmt.Sprintf("%s\n%s\n%s:\n%s",
		summary, tools.LineSep, contextLineNumbers, string(contextOutput))
}
