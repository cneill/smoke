package ls

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/cneill/smoke/pkg/fs"
	"github.com/cneill/smoke/pkg/tools"
)

const (
	ParamDirPath = "dir_path"
)

type LS struct {
	ProjectPath string
}

func New(projectPath, _ string) (tools.Tool, error) {
	return &LS{ProjectPath: projectPath}, nil
}

func (l *LS) Name() string { return tools.NameLS }
func (l *LS) Description() string {
	examples := tools.CollectExamples(l.Examples()...)

	return fmt.Sprintf("List files in the directory %q recursively, with file mode + size info.%s",
		ParamDirPath, examples)
}

func (l *LS) Examples() tools.Examples {
	return tools.Examples{
		{
			Description: `List all files in the "pkg/models" directory recursively`,
			Args:        tools.Args{ParamDirPath: "pkg/models"},
		},
	}
}

func (l *LS) Params() tools.Params {
	return tools.Params{
		{
			Key:         ParamDirPath,
			Description: "The path to the directory where you want to list files",
			Type:        "string",
			Required:    true,
		},
	}
}

// Run expects a directory 'dir' that exists within ProjectPath ("." for top-level listing).
func (l *LS) Run(_ context.Context, args tools.Args) (*tools.Output, error) {
	path := args.GetString(ParamDirPath)
	if path == nil {
		return nil, fmt.Errorf("no path supplied")
	}

	fullPath, err := fs.GetRelativePath(l.ProjectPath, *path)
	if err != nil {
		return nil, fmt.Errorf("path error: %w", err)
	}

	// TODO: check for non-directory, symlink/escape/etc when fs rewritten

	var sb strings.Builder

	iter, err := fs.ExcludesWalker(l.ProjectPath, fullPath)
	if err != nil {
		return nil, fmt.Errorf("failed to walk files: %w", err)
	}

	for entry, err := range iter {
		if err != nil {
			slog.Error("walk error in list_files", "target_path", fullPath, "error", err)
			continue
		}

		info, err := entry.DirEntry.Info()
		if err != nil {
			return nil, fmt.Errorf("failed to get info about path %q: %w", entry.RelPath, err)
		}

		fmt.Fprintf(&sb, "[%s] %dB %s\n", info.Mode().String(), info.Size(), entry.RelPath)
	}

	content := sb.String()
	if strings.TrimSpace(content) == "" {
		content = fmt.Sprintf("Provided path %q is empty.", *path)
	}

	return &tools.Output{Text: content}, nil
}
