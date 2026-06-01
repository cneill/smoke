package ignores

import (
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"

	"github.com/go-git/go-git/v5/plumbing/format/gitignore"
)

const (
	IgnoreDotName = ".smokeignore"
	IgnoreName    = "ignore"
)

type Opts struct {
	ConfigDir  string
	ProjectDir string
}

func (o Opts) OK() error {
	switch {
	case o.ConfigDir != "" && !filepath.IsAbs(o.ConfigDir):
		return fmt.Errorf("config dir %q is not absolute", o.ConfigDir)
	case o.ProjectDir != "" && !filepath.IsAbs(o.ProjectDir):
		return fmt.Errorf("project dir %q is not absolute", o.ProjectDir)
	}

	return nil
}

type Ignorer struct {
	configDir  string
	projectDir string
	files      []IgnoreFile
	matcher    gitignore.Matcher
}

func NewIgnorer(opts Opts) (*Ignorer, error) {
	if err := opts.OK(); err != nil {
		return nil, fmt.Errorf("ignorer option error: %w", err)
	}

	ignoreFiles := GetIgnoreFiles(opts.ConfigDir, opts.ProjectDir)

	ignorer := &Ignorer{
		configDir:  opts.ConfigDir,
		projectDir: opts.ProjectDir,
		files:      ignoreFiles,
	}

	ignorer.Refresh()

	return ignorer, nil
}

func (i *Ignorer) Refresh() {
	var ignorePatterns []gitignore.Pattern

	for _, file := range i.files {
		filePatterns, err := file.Patterns()
		if err != nil {
			slog.Error("failed to load ignore patterns from ignore file", "path", file.Path, "error", err)
		}

		ignorePatterns = append(ignorePatterns, filePatterns...)
	}

	i.matcher = gitignore.NewMatcher(ignorePatterns)
}

func (i *Ignorer) Ignored(path string, isDir bool) bool {
	return i.matcher.Match(pathSegments(path), isDir)
}

func GetIgnoreFiles(configDir, projectDir string) []IgnoreFile {
	var ignoreFiles []IgnoreFile

	if configDir != "" {
		userPath := filepath.Join(configDir, IgnoreName)

		userFile, err := ParseIgnoreFile(userPath, false)
		if err != nil {
			slog.Debug("skipping user ignores file", "path", userPath, "error", err)
		} else {
			ignoreFiles = append(ignoreFiles, userFile)
		}
	}

	if projectDir != "" {
		projectRootPath := filepath.Join(projectDir, IgnoreDotName)

		projectRootFile, err := ParseIgnoreFile(projectRootPath, true)
		if err != nil {
			slog.Debug("skipping project-level ignores file", "path", projectRootPath, "error", err)
		} else {
			ignoreFiles = append(ignoreFiles, projectRootFile)
		}
	}

	// TODO: capture nested project ignores files

	return ignoreFiles
}

func pathSegments(path string) []string {
	if path == "" {
		return []string{}
	}

	var pathSegments []string

	for part := range strings.SplitSeq(filepath.ToSlash(path), "/") {
		if part == "" {
			continue
		}

		pathSegments = append(pathSegments, part)
	}

	return pathSegments
}
