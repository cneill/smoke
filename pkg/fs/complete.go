package fs

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// PathMatch is a single filesystem entry relative to the project root that matches a path-completion query.
// Directory Path values always end with a trailing slash.
type PathMatch struct {
	// Path is the project-relative path, using forward slashes. Directories end with "/".
	Path string
	// IsDir is true when the entry is a directory.
	IsDir bool
}

// PathCompleter returns a function that lists project-relative path matches for a partial query.
// The query is expected without a leading "@". Matching is prefix-based on the final path segment
// and only inspects a single directory level (not a recursive walk). Results honor .smokeignore
// via GetExcludeMatcher and never escape projectPath. Errors (including insecure paths) yield nil.
func PathCompleter(projectPath string) func(string) []PathMatch {
	return func(query string) []PathMatch {
		matches, err := ListPathMatches(projectPath, query)
		if err != nil {
			return nil
		}

		return matches
	}
}

// ListPathMatches returns project-relative path entries that match query under projectPath.
// query uses forward or native separators; a trailing "/" means "list children of this directory".
// Empty query yields no matches (callers require at least one character after "@").
// Traversal attempts and missing directories yield empty results (not errors) so completion UX stays quiet.
func ListPathMatches(projectPath, query string) ([]PathMatch, error) {
	if err := validateProjectPath(projectPath); err != nil {
		return nil, err
	}

	query = strings.ReplaceAll(query, "\\", "/")
	if query == "" || hasTraversal(query) {
		return nil, nil
	}

	dirRel, prefix := filepath.Split(query)
	if dirRel != "" && hasTraversal(dirRel) {
		return nil, nil
	}

	dirAbs, err := GetRelativePath(projectPath, dirRel)
	if err != nil {
		return nil, nil //nolint:nilerr
	}

	entries, err := os.ReadDir(dirAbs)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}

		return nil, fmt.Errorf("failed to read directory %q: %w", dirRel, err)
	}

	return filterPathEntries(projectPath, dirRel, prefix, entries), nil
}

func filterPathEntries(projectPath, dirRel, prefix string, entries []os.DirEntry) []PathMatch {
	excludes := GetExcludeMatcher(projectPath)
	results := make([]PathMatch, 0, len(entries))

	for _, entry := range entries {
		name := entry.Name()
		if prefix != "" && !strings.HasPrefix(name, prefix) {
			continue
		}

		relPath := name
		if dirRel != "" {
			relPath = dirRel + name
		}

		absPath := filepath.Join(projectPath, filepath.FromSlash(relPath))
		isDir := entry.IsDir()

		if excludes.Match(absPath, isDir) {
			continue
		}

		display := relPath
		if isDir {
			display += "/"
		}

		results = append(results, PathMatch{
			Path:  display,
			IsDir: isDir,
		})
	}

	slices.SortFunc(results, func(a, b PathMatch) int {
		return strings.Compare(a.Path, b.Path)
	})

	return results
}
