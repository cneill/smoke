package ignores

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/cneill/smoke/pkg/files/paths"
	"github.com/go-git/go-git/v5/plumbing/format/gitignore"
)

type IgnoreFile struct {
	Path        string
	PathScoped  bool
	dirSegments []string
}

func ParseIgnoreFile(path string, pathScoped bool) (IgnoreFile, error) {
	if !filepath.IsAbs(path) {
		return IgnoreFile{}, fmt.Errorf("path %q is not absolute", path)
	}

	exists, err := paths.OptionalRegularFile(path)
	if err != nil {
		return IgnoreFile{}, fmt.Errorf("invalid ignores file %q: %w", path, err)
	} else if !exists {
		return IgnoreFile{}, fmt.Errorf("file %q does not exist", path)
	}

	var dirSegments []string

	// gitignore expects a []string of path segments if the ignore file patterns are intended to be path-scoped (i.e.,
	// they're not global patterns).
	if pathScoped {
		dirSegments = pathSegments(filepath.Dir(path))
	}

	return IgnoreFile{
		Path:        path,
		PathScoped:  pathScoped,
		dirSegments: dirSegments,
	}, nil
}

// Patterns reads the ignore file and collects + parses its patterns with gitignore.
func (i IgnoreFile) Patterns() ([]gitignore.Pattern, error) {
	var patterns []gitignore.Pattern

	file, err := os.Open(i.Path)
	if err != nil {
		return nil, fmt.Errorf("opening ignores file: %w", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		if err := scanner.Err(); err != nil {
			return nil, fmt.Errorf("reading ignores file: %w", err)
		}

		line := scanner.Text()

		if trimmedLine := strings.TrimSpace(line); trimmedLine == "" || strings.HasPrefix(trimmedLine, "#") {
			continue
		}

		pattern := gitignore.ParsePattern(line, i.dirSegments)
		patterns = append(patterns, pattern)
	}

	return patterns, nil
}
