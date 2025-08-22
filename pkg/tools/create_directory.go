package tools

import (
	"context"
	"fmt"
	"os"

	"github.com/cneill/smoke/pkg/fs"
)

const (
	CreateDirectoryPath = "path"
)

type CreateDirectoryTool struct {
	ProjectPath string
}

func NewCreateDirectoryTool(projectPath, _ string) Tool {
	return &CreateDirectoryTool{
		ProjectPath: projectPath,
	}
}

var _ = Tool(&CreateDirectoryTool{})

func (c *CreateDirectoryTool) Name() string { return ToolCreateDirectory }
func (c *CreateDirectoryTool) Description() string {
	return "Create a new directory at the given path"
}

func (c *CreateDirectoryTool) Params() Params {
	return Params{
		{
			Key:         CreateDirectoryPath,
			Description: "The path where the directory should be created",
			Type:        ParamTypeString,
			Required:    true,
		},
	}
}

func (c *CreateDirectoryTool) Run(_ context.Context, args Args) (string, error) {
	path := args.GetString(CreateDirectoryPath)
	if path == nil {
		return "", fmt.Errorf("%w: no path supplied", ErrArguments)
	}

	fullPath, err := fs.GetRelativePath(c.ProjectPath, *path)
	if err != nil {
		return "", fmt.Errorf("%w: path error: %w", err, ErrArguments)
	}

	err = os.MkdirAll(fullPath, 0o755)
	if err != nil {
		return "", fmt.Errorf("%w: failed to create directory %q: %w", ErrFileSystem, fullPath, err)
	}

	return fmt.Sprintf("Created directory at %q", *path), nil
}
