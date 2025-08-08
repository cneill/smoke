package tools

import (
	"bufio"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/cneill/smoke/pkg/utils"
)

const (
	GrepPath         = "path"
	GrepRegex        = "regex"
	GrepContextLines = "context_lines"
)

type GrepTool struct {
	ProjectPath string
}

func (g *GrepTool) Name() string { return "grep" }
func (g *GrepTool) Description() string {
	return "Search a file or directory for a regular expression. Does not search recursively."
}

func (g *GrepTool) Params() Params {
	return Params{
		{
			Key:         GrepPath,
			Description: "The path (either a directory or file) to search for the regex",
			Type:        ParamTypeString,
			Required:    true,
		},
		{
			Key:         GrepRegex,
			Description: "The regular expression (in Golang regexp syntax) to search for",
			Type:        ParamTypeString,
			Required:    true,
		},
		{
			Key:         GrepContextLines,
			Description: "The number of lines of context to provide around matches. If empty/0, defaults to only matched lines.",
			Type:        ParamTypeNumber,
			Required:    false,
		},
	}
}

func (g *GrepTool) Run(args Args) (string, error) {
	path := args.GetString(GrepPath)
	if path == nil {
		return "", fmt.Errorf("no path supplied")
	}

	fullPath, err := utils.GetRelativePath(g.ProjectPath, *path)
	if err != nil {
		return "", fmt.Errorf("path error: %w", err)
	}

	regex := args.GetString(GrepRegex)
	if regex == nil {
		return "", fmt.Errorf("no regex supplied")
	}

	compiled, err := regexp.Compile(*regex)
	if err != nil {
		return "", fmt.Errorf("failed to compile regex pattern: %w", err)
	}

	var contextLines int64

	contextLinesPtr := args.GetInt64(GrepContextLines)
	if contextLinesPtr != nil && *contextLinesPtr > 0 {
		contextLines = *contextLinesPtr
	}

	stat, err := os.Stat(fullPath)
	if err != nil {
		return "", fmt.Errorf("failed to stat path %q: %w", fullPath, err)
	}

	if !stat.Mode().IsRegular() {
		return "", fmt.Errorf("invalid file for grep: %q (mode=%s)", fullPath, stat.Mode().String())
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

	for filePath, fileResults := range dirResults {
		relPath, err := filepath.Rel(g.ProjectPath, filePath)
		if err != nil {
			return "", fmt.Errorf("invalid file path %q: %w", filePath, err)
		}

		output += relPath + "\n---------\n"
		for _, result := range fileResults {
			output += strings.Join(result, "\n") + "\n\n"
		}
	}

	return output, nil
}

func (g *GrepTool) getFileResults(fullPath string, pattern *regexp.Regexp, contextLines int64) ([][]string, error) {
	file, err := os.Open(fullPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file %q: %w", fullPath, err)
	}
	defer file.Close()

	var (
		lineNum int64
		lines   = []string{}
		results = [][]string{}
		scanner = bufio.NewScanner(file)
	)

	for scanner.Scan() {
		lineNum++
		line := scanner.Text()
		lines = append(lines, line)
		pendingMatch := []string{}

		if pattern.MatchString(line) {
			context := []string{}

			if contextLines > 0 {
				for i := max(0, lineNum-1-contextLines); i < lineNum-1; i++ {
					context = append(context, fmt.Sprintf("%d: %s", i+1, lines[i]))
				}
			}

			context = append(context, fmt.Sprintf("*%d: %s", lineNum, line))
			// results = append(results, context)
			pendingMatch = context
		}

		for i := range results {
			if int64(len(results[i])) == 2*contextLines+1 {
				continue
			}

			results[i] = append(results[i], fmt.Sprintf("%d: %s", lineNum, line))
		}

		if len(pendingMatch) > 0 {
			results = append(results, pendingMatch)
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("failed to read file %q: %w", fullPath, err)
	}

	return results, nil
}

func (g *GrepTool) getDirResults(fullPath string, pattern *regexp.Regexp, contextLines int64) (map[string][][]string, error) {
	results := map[string][][]string{}

	walkErr := filepath.WalkDir(fullPath, func(path string, dirEntry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if dirEntry.IsDir() {
			return fs.SkipDir
		}

		info, err := dirEntry.Info()
		if err != nil {
			return fmt.Errorf("failed to stat %q: %w", path, err)
		}

		if info.Mode().IsRegular() {
			fileResults, err := g.getFileResults(path, pattern, contextLines)
			if err != nil {
				slog.Error("failed to grep file", "path", path, "error", err)
			}

			if len(fileResults) > 0 {
				results[path] = fileResults
			}
		}

		return nil
	})

	if walkErr != nil {
		return nil, fmt.Errorf("failed to grep directory %q: %w", fullPath, walkErr)
	}

	return results, nil
}
