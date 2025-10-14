package listfiles

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"strings"

	"github.com/cneill/smoke/pkg/fs"
	"github.com/cneill/smoke/pkg/tools"
)

const (
	Name      = "list_files"
	PathParam = "path"
)

type ListFiles struct {
	ProjectPath string
}

func New(projectPath, _ string) tools.Tool {
	return &ListFiles{ProjectPath: projectPath}
}

func (l *ListFiles) Name() string { return Name }
func (l *ListFiles) Description() string {
	examples := tools.CollectExamples(l.Examples()...)

	return fmt.Sprintf("List files in the directory %q recursively, with file mode + size info.%s",
		PathParam, examples)
}

func (l *ListFiles) Examples() tools.Examples {
	return tools.Examples{
		{
			Description: `List all files in the "pkg/models" directory recursively`,
			Args:        tools.Args{PathParam: "pkg/models"},
		},
	}
}

func (l *ListFiles) Params() tools.Params {
	return tools.Params{
		{
			Key:         PathParam,
			Description: "The path to the directory where you want to list files",
			Type:        "string",
			Required:    true,
		},
	}
}

// Run expects a directory 'dir' that exists within ProjectPath ("." for top-level listing).
func (l *ListFiles) Run(_ context.Context, args tools.Args) (string, error) {
	path := args.GetString(PathParam)
	if path == nil {
		return "", fmt.Errorf("no path supplied")
	}

	fullPath, err := fs.GetRelativePath(l.ProjectPath, *path)
	if err != nil {
		return "", fmt.Errorf("path error: %w", err)
	}

	builder := strings.Builder{}

	iter, err := fs.ExcludesWalker(l.ProjectPath, fullPath)
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
			return "", fmt.Errorf("failed to get info about path %q: %w", entry.RelPath, err)
		}

		if _, err := builder.WriteString("[" + info.Mode().String() + "] "); err != nil {
			return "", fmt.Errorf("failed to add file mode: %w", err)
		}

		if _, err := builder.WriteString(strconv.FormatInt(info.Size(), 10) + "B "); err != nil {
			return "", fmt.Errorf("failed to add file size: %w", err)
		}

		if _, err := builder.WriteString("/" + entry.RelPath + "\n"); err != nil {
			return "", fmt.Errorf("failed to add path: %w", err)
		}
	}

	output := builder.String()

	return output, nil
}
