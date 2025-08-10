package tools

import (
	"fmt"
	"os"

	"github.com/cneill/smoke/pkg/utils"
)

const (
	CreateDirectoryPath = "path"
)

type CreateDirectoryTool struct {
	ProjectPath string
}

var _ = Tool(&CreateDirectoryTool{})

func (c *CreateDirectoryTool) Name() string { return ToolCreateDirectory }
func (c *CreateDirectoryTool) Description() string {
	return "Create a new directory at the given path."
}

func (c *CreateDirectoryTool) Params() Params {
	return Params{
		{
			Key:         CreateDirectoryPath,
			Description: "The path where the directory should be created.",
			Type:        ParamTypeString,
			Required:    true,
		},
	}
}

func (c *CreateDirectoryTool) Run(args Args) (string, error) {
	path := args.GetString(CreateDirectoryPath)
	if path == nil {
		return "", fmt.Errorf("no path supplied")
	}

	fullPath, err := utils.GetRelativePath(c.ProjectPath, *path)
	if err != nil {
		return "", fmt.Errorf("path error: %w", err)
	}

	err = os.MkdirAll(fullPath, 0o755)
	if err != nil {
		return "", fmt.Errorf("failed to create directory %q: %w", fullPath, err)
	}

	return fmt.Sprintf("Created directory at %q", *path), nil
}
