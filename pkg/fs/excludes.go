package fs

import (
	"bufio"
	"errors"
	"fmt"
	"io/fs"
	"iter"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-git/go-git/v5/plumbing/format/gitignore"
)

const smokeIgnore = ".smokeignore"

// ExcludeMatcher wraps the gitignore Matcher from go-git and handles the path-splitting it requires automatically.
type ExcludeMatcher struct {
	matcher gitignore.Matcher
}

func (m ExcludeMatcher) Match(path string, isDir bool) bool {
	return m.matcher.Match(getParts(path), isDir)
}

var excludesFilePaths = map[string][]string{} //nolint:gochecknoglobals // used for caching

// WalkerEntry has the details returned by a single iteration of an fs.WalkDirFunc call during filepath.WalkDir.
type WalkerEntry struct {
	Path     string
	RelPath  string
	DirEntry fs.DirEntry
}

// ExcludesWalker returns an iterator that yields the entries from a filepath.WalkDir call, filtered by the
// ExcludesMatcher for 'targetPath'.
func ExcludesWalker(projectPath, targetPath string) (iter.Seq2[WalkerEntry, error], error) {
	absPath, err := getWalkPath(projectPath, targetPath)
	if err != nil {
		return nil, fmt.Errorf("error with path: %w", err)
	}

	excludes := GetExcludeMatcher(projectPath)
	errStop := errors.New("stop walk")

	iter := func(yield func(WalkerEntry, error) bool) {
		_ = filepath.WalkDir(absPath, func(path string, dirEntry fs.DirEntry, err error) error {
			if err != nil {
				if !yield(WalkerEntry{}, fmt.Errorf("walk error on %q: %w", path, err)) {
					return errStop
				}

				return nil
			}

			if path == absPath {
				return nil
			}

			isDir := dirEntry.IsDir()
			if exclude := excludes.Match(path, isDir); exclude {
				if isDir {
					return fs.SkipDir
				}

				return nil
			}

			relPath, err := filepath.Rel(projectPath, path)
			if err != nil {
				if !yield(WalkerEntry{}, fmt.Errorf("failed to get relative path for %q: %w", path, err)) {
					return errStop
				}
			}

			entry := WalkerEntry{
				Path:     path,
				RelPath:  relPath,
				DirEntry: dirEntry,
			}

			if !yield(entry, nil) {
				return errStop
			}

			return nil
		})
	}

	return iter, nil
}

// getWalkPath checks that "targetPath" is part of "projectPath" and ensures it is an absolute path.
func getWalkPath(projectPath, targetPath string) (string, error) {
	result := targetPath

	if _, err := filepath.Rel(projectPath, targetPath); err != nil {
		return "", fmt.Errorf("target path is not a subpath of project path: %w", err)
	}

	if !filepath.IsAbs(targetPath) {
		abs, err := filepath.Abs(targetPath)
		if err != nil {
			return "", fmt.Errorf("failed to get absolute path from %q: %w", targetPath, err)
		}

		result = abs
	}

	return result, nil
}

// GetExcludeMatcher looks at a few locations to get exclusion files, then returns an ExcludeMatcher that will exclude
// files referenced within them. Those files are:
//
// -  [projectPath]/.smokeignore
// - $HOME/.smokeignore
// - $XDG_CONFIG_HOME/smoke/ignore
// - $HOME/config/smoke/ignore (if no $XDG_CONFIG_HOME defined)
func GetExcludeMatcher(projectPath string) ExcludeMatcher {
	paths, ok := excludesFilePaths[projectPath]
	if !ok {
		paths = getExcludesFilePaths(projectPath)
		excludesFilePaths[projectPath] = paths
	}

	patterns := []gitignore.Pattern{}

	for _, path := range paths {
		stat, err := os.Stat(path)
		if err != nil {
			// TODO: error if something other than "not exist"?
			continue
		}

		if stat.IsDir() {
			continue
		}

		pathPatterns := readFilePatterns(path)
		if pathPatterns != nil {
			patterns = append(patterns, pathPatterns...)
		}
	}

	return ExcludeMatcher{gitignore.NewMatcher(patterns)}
}

func getParts(path string) []string {
	results := []string{}

	for part := range strings.SplitSeq(filepath.ToSlash(path), "/") {
		if part == "" {
			continue
		}

		results = append(results, part)
	}

	return results
}

func readFilePatterns(path string) []gitignore.Pattern {
	results := []gitignore.Pattern{}

	dir, _ := filepath.Split(path)
	dirParts := getParts(dir)

	slog.Debug("reading smoke ignore patterns", "file", path, "dir", dirParts)

	file, err := os.Open(path)
	if err != nil {
		// TODO: error if something other than "not exist"?
		return nil
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		pattern := gitignore.ParsePattern(line, dirParts)
		results = append(results, pattern)
	}

	return results
}

func getExcludesFilePaths(projectPath string) []string {
	results := []string{}

	// [project_path]/.smokeignore
	repoPath, err := GetRelativePath(projectPath, smokeIgnore)
	if err == nil {
		results = append(results, repoPath)
	}

	// $HOME/.smokeignore
	home, _ := os.UserHomeDir()
	if home != "" {
		results = append(results, filepath.Join(home, smokeIgnore))
	}

	// $XDG_CONFIG_HOME/smoke/ignore OR $HOME/.config/smoke/ignore
	if xdgConfig := os.Getenv("XDG_CONFIG_HOME"); xdgConfig != "" {
		results = append(results, filepath.Join(xdgConfig, "smoke", "ignore"))
	} else if home != "" {
		results = append(results, filepath.Join(home, ".config", "smoke", "ignore"))
	}

	return results
}
