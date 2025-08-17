package tools

import (
	"context"
	"fmt"
	"io/fs"
	"log/slog"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/cneill/smoke/pkg/utils"
)

const (
	ListFilesPath = "path"
)

//nolint:gochecknoglobals
var ignoredDirectories = []string{
	".git",
}

type ListFilesTool struct {
	ProjectPath string
}

var _ = Tool(&ListFilesTool{})

func (l *ListFilesTool) Name() string { return ToolListFiles }
func (l *ListFilesTool) Description() string {
	return "List files in the given directory, recursively."
}

func (l *ListFilesTool) Params() Params {
	return Params{
		{
			Key:      ListFilesPath,
			Type:     "string",
			Required: true,
		},
	}
}

// ListFiles expects a directory 'dir' that exists within ProjectPath.
// TODO: .gitignore?
func (l *ListFilesTool) Run(_ context.Context, args Args) (string, error) {
	path := args.GetString(ListFilesPath)
	if path == nil {
		return "", fmt.Errorf("no path supplied")
	}

	fullPath, err := utils.GetRelativePath(l.ProjectPath, *path)
	if err != nil {
		return "", fmt.Errorf("path error: %w", err)
	}

	builder := strings.Builder{}

	walkErr := filepath.WalkDir(fullPath, walker(l.ProjectPath, &builder))
	if walkErr != nil {
		return "", fmt.Errorf("walk error: %w", walkErr)
	}

	output := builder.String()

	return output, nil
}

func walker(projectPath string, builder *strings.Builder) fs.WalkDirFunc {
	return func(path string, dirEntry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if dirEntry.IsDir() {
			// Don't include e.g. ".git" directory contents returned as output
			if slices.Contains(ignoredDirectories, dirEntry.Name()) {
				slog.Debug("list_files encountered ignored directory", "path", path)
				return fs.SkipDir
			}
		}

		info, err := dirEntry.Info()
		if err != nil {
			return fmt.Errorf("failed to get info about path %q: %w", path, err)
		}

		if _, err := builder.WriteString("[" + info.Mode().String() + "] "); err != nil {
			return fmt.Errorf("failed to add file mode: %w", err)
		}

		if _, err := builder.WriteString(strconv.FormatInt(info.Size(), 10) + "B "); err != nil {
			return fmt.Errorf("failed to add file size: %w", err)
		}

		relPath, err := filepath.Rel(projectPath, path)
		if err != nil {
			return fmt.Errorf("invalid file path %q: %w", path, err)
		}

		if _, err := builder.WriteString("/" + relPath + "\n"); err != nil {
			return fmt.Errorf("failed to add path: %w", err)
		}

		return nil
	}
}
