package tools

import (
	"errors"
	"fmt"
	"io/fs"
	"os"

	"github.com/cneill/smoke/pkg/utils"
)

const (
	WriteFilePath      = "path"
	WriteFileContents  = "contents"
	WriteFileOverwrite = "overwrite"
)

type WriteFileTool struct {
	ProjectPath string
}

var _ = Tool(&WriteFileTool{})

func (w *WriteFileTool) Name() string { return ToolWriteFile }
func (w *WriteFileTool) Description() string {
	return "Create a new file and write the specified contents to it. Cannot edit existing files."
}

func (w *WriteFileTool) Params() Params {
	return Params{
		{
			Key:         WriteFilePath,
			Description: "The path of the file to rewrite",
			Type:        ParamTypeString,
			Required:    true,
		},
		{
			Key:         WriteFileContents,
			Description: "The contents to write to the specified file",
			Type:        ParamTypeString,
			Required:    true,
		},
	}
}

func (w *WriteFileTool) Run(args Args) (string, error) {
	path := args.GetString(WriteFilePath)
	if path == nil {
		return "", fmt.Errorf("no path supplied")
	}

	fullPath, err := utils.GetRelativePath(w.ProjectPath, *path)
	if err != nil {
		return "", fmt.Errorf("path error: %w", err)
	}

	contents := args.GetString(WriteFileContents)
	if contents == nil {
		return "", fmt.Errorf("no contents supplied")
	}

	_, statErr := os.Stat(fullPath)
	if statErr != nil && !errors.Is(statErr, fs.ErrNotExist) {
		return "", fmt.Errorf("failed to stat %q: %w", *path, statErr)
	}

	if statErr == nil {
		return "", fmt.Errorf("refusing to overwrite %q", *path)
	}

	if err := os.WriteFile(fullPath, []byte(*contents), 0o644); err != nil {
		return "", fmt.Errorf("failed to write contents to %q: %w", *path, err)
	}

	return fmt.Sprintf("Wrote contents to %q", *path), nil
}
