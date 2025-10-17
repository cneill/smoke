package grep

import (
	"bufio"
	"context"
	"fmt"
	"log/slog"
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/cneill/smoke/pkg/fs"
	"github.com/cneill/smoke/pkg/tools"
	"github.com/cneill/smoke/pkg/tools/formatting"
)

const (
	ParamPath         = "path"
	ParamRegex        = "regex"
	ParamContextLines = "context_lines"
)

type Grep struct {
	ProjectPath string
}

func New(projectPath, _ string) (tools.Tool, error) {
	return &Grep{ProjectPath: projectPath}, nil
}

func (g *Grep) Name() string { return tools.NameGrep }
func (g *Grep) Description() string {
	examples := tools.CollectExamples(g.Examples()...)

	return fmt.Sprintf(`Search a file or directory for a regular expression. Lines matching %q are prefixed with "*", `+
		"while context lines that do not include matches only include line numbers. Optionally, provide %q for "+
		"the number of lines of context (default 0, only matched lines). Does not match multi-line regexes.%s",
		ParamRegex, ParamContextLines, examples,
	)
}

func (g *Grep) Examples() tools.Examples {
	return tools.Examples{
		{
			Description: `Search for the regex "type.*Tool" in the "pkg/tools" directory with no extra context lines.`,
			Args: tools.Args{
				ParamPath:  "pkg/tools",
				ParamRegex: "type.*Tool",
			},
		},
		{
			Description: `Search for the regex "type.*Client" in the whole repository with 3 lines of context on either side of each match.`,
			Args: tools.Args{
				ParamPath:         ".",
				ParamRegex:        "type.*Client",
				ParamContextLines: 3,
			},
		},
	}
}

func (g *Grep) Params() tools.Params {
	return tools.Params{
		{
			Key:         ParamPath,
			Description: "The path of the file/directory to search for the regex",
			Type:        tools.ParamTypeString,
			Required:    true,
		},
		{
			Key:         ParamRegex,
			Description: "The regular expression (in Golang regexp syntax) to search for. No multi-line regexes.",
			Type:        tools.ParamTypeString,
			Required:    true,
		},
		{
			Key: ParamContextLines,
			Description: "The number of lines of context to provide around matches. If empty/0, defaults to only " +
				"returning matched lines",
			Type:     tools.ParamTypeNumber,
			Required: false,
		},
	}
}

func (g *Grep) Run(_ context.Context, args tools.Args) (string, error) {
	path := args.GetString(ParamPath)
	if path == nil {
		return "", fmt.Errorf("%w: no path supplied", tools.ErrArguments)
	}

	fullPath, err := fs.GetRelativePath(g.ProjectPath, *path)
	if err != nil {
		return "", fmt.Errorf("%w: path error: %w", tools.ErrArguments, err)
	}

	regex := args.GetString(ParamRegex)
	if regex == nil || *regex == "" {
		return "", fmt.Errorf("%w: no regex or empty regex supplied", tools.ErrArguments)
	}

	compiled, err := regexp.Compile(*regex)
	if err != nil {
		return "", fmt.Errorf("%w: failed to compile regex pattern: %w", tools.ErrArguments, err)
	}

	var contextLines int64

	if contextLinesPtr := args.GetInt64(ParamContextLines); contextLinesPtr != nil {
		if *contextLinesPtr < 0 {
			return "", fmt.Errorf("%w: %q must be >=0", tools.ErrArguments, ParamContextLines)
		}

		contextLines = *contextLinesPtr
	}

	stat, err := os.Stat(fullPath)
	if err != nil {
		return "", fmt.Errorf("%w: failed to stat path %q: %w", tools.ErrFileSystem, fullPath, err)
	}

	if !stat.Mode().IsRegular() && !stat.IsDir() {
		return "", fmt.Errorf("%w: invalid file for grep: %q (mode=%s)", tools.ErrFileSystem, fullPath, stat.Mode().String())
	}

	dirResults := map[string][][]string{}

	if !stat.IsDir() {
		fileResults, err := g.getFileResults(fullPath, compiled, contextLines)
		if err != nil {
			return "", err
		}

		dirResults[fullPath] = fileResults
	} else {
		dirResults, err = g.getDirResults(fullPath, compiled, contextLines)
		if err != nil {
			return "", err
		}
	}

	var output string

	sortedPaths := slices.Collect(maps.Keys(dirResults))
	slices.Sort(sortedPaths)

	for _, filePath := range sortedPaths {
		fileResults := dirResults[filePath]

		relPath, err := filepath.Rel(g.ProjectPath, filePath)
		if err != nil {
			return "", fmt.Errorf("%w: invalid file path %q: %w", tools.ErrFileSystem, filePath, err)
		}

		output += relPath + "\n" + formatting.LineSep + "\n"
		for _, result := range fileResults {
			output += strings.Join(result, "\n") + "\n\n"
		}
	}

	return output, nil
}

func (g *Grep) getFileResults(fullPath string, pattern *regexp.Regexp, contextLines int64) ([][]string, error) {
	file, err := os.Open(fullPath)
	if err != nil {
		return nil, fmt.Errorf("%w: failed to open file %q: %w", tools.ErrFileSystem, fullPath, err)
	}
	defer file.Close()

	var (
		// lineNum int64
		lines   = []string{}
		results = [][]string{}
		scanner = bufio.NewScanner(file)
	)

	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}

	// TODO: test for / fix max line size issue for extremely long lines?
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("%w: failed to read file %q: %w", tools.ErrFileSystem, fullPath, err)
	}

	for lineNum, line := range lines {
		if pattern.MatchString(line) {
			context := []string{}

			// add the context preceding the match, if requested
			if contextLines > 0 {
				start := max(0, int64(lineNum)-contextLines)

				for i := start; i < int64(lineNum); i++ {
					context = append(context, fmt.Sprintf("%d: %s", i+1, lines[i]))
				}
			}

			// add the matched line, prefixing its line number with '*'
			context = append(context, fmt.Sprintf("*%d: %s", lineNum+1, line))

			// add the context following the match, if requested
			if contextLines > 0 {
				end := min(int64(len(lines)), int64(lineNum+1)+contextLines)

				for i := lineNum + 1; int64(i) < end; i++ {
					context = append(context, fmt.Sprintf("%d: %s", i+1, lines[i]))
				}
			}

			results = append(results, context)
		}
	}

	return results, nil
}

func (g *Grep) getDirResults(fullPath string, pattern *regexp.Regexp, contextLines int64) (map[string][][]string, error) {
	results := map[string][][]string{}

	iter, err := fs.ExcludesWalker(g.ProjectPath, fullPath)
	if err != nil {
		return nil, fmt.Errorf("failed to grep directory %q: %w", fullPath, err)
	}

	for entry, err := range iter {
		if err != nil {
			slog.Error("walk error in grep", "target_path", fullPath, "error", err)
			continue
		}

		info, err := entry.DirEntry.Info()
		if err != nil {
			slog.Error("error getting info for path", "path", entry.Path, "error", err)
			continue
		}

		if !info.Mode().IsRegular() {
			continue
		}

		fileResults, err := g.getFileResults(entry.Path, pattern, contextLines)
		if err != nil {
			slog.Error("failed to grep file", "path", entry.Path, "error", err)
			continue
		}

		if len(fileResults) > 0 {
			results[entry.Path] = fileResults
		}
	}

	return results, nil
}
