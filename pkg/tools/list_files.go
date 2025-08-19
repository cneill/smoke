package tools

import (
	"context"
	"fmt"
	"log/slog"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/cneill/smoke/pkg/fs"
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

	fullPath, err := fs.GetRelativePath(l.ProjectPath, *path)
	if err != nil {
		return "", fmt.Errorf("path error: %w", err)
	}

	builder := strings.Builder{}

	iter, err := fs.ExcludesWalker(fullPath)
	if err != nil {
		return "", fmt.Errorf("failed to walk files: %w", err)
	}

	for entry, err := range iter {
		if err != nil {
			slog.Error("walk error in list_files", "target_path", fullPath, "error", err)
			continue
		}

		info, err := entry.DirEntry.Info()
		if err != nil {
			return "", fmt.Errorf("failed to get info about path %q: %w", entry.Path, err)
		}

		if _, err := builder.WriteString("[" + info.Mode().String() + "] "); err != nil {
			return "", fmt.Errorf("failed to add file mode: %w", err)
		}

		if _, err := builder.WriteString(strconv.FormatInt(info.Size(), 10) + "B "); err != nil {
			return "", fmt.Errorf("failed to add file size: %w", err)
		}

		relPath, err := filepath.Rel(l.ProjectPath, entry.Path)
		if err != nil {
			return "", fmt.Errorf("invalid file path %q: %w", entry.Path, err)
		}

		if _, err := builder.WriteString("/" + relPath + "\n"); err != nil {
			return "", fmt.Errorf("failed to add path: %w", err)
		}
	}

	output := builder.String()

	return output, nil
}
