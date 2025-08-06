package tools

import (
	"errors"
	"fmt"
	"io/fs"
	"os"

	"github.com/cneill/smack/pkg/paths"
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
	return "Replace the contents of the file at the given path with the contents provided in this call"
}

func (w *WriteFileTool) Params() Params {
	return Params{
		{
			Key:         WriteFilePath,
			Description: "The path of the file to rewrite.",
			Type:        ParamTypeString,
			Required:    true,
		},
		{
			Key:         WriteFileContents,
			Description: "The contents to write to the file at path.",
			Type:        ParamTypeString,
			Required:    true,
		},
		{
			Key:         WriteFileOverwrite,
			Description: "Overwrite the file at the given path if it already exists. Required for rewrites.",
			Type:        ParamTypeBoolean,
			Required:    false,
		},
	}
}

func (w *WriteFileTool) Run(args Args) (string, error) {
	path := args.GetString(WriteFilePath)
	if path == nil {
		return "", fmt.Errorf("no path supplied")
	}

	fullPath, err := paths.GetRelative(w.ProjectPath, *path)
	if err != nil {
		return "", fmt.Errorf("path error: %w", err)
	}

	contents := args.GetString(WriteFileContents)
	if contents == nil {
		return "", fmt.Errorf("no contents supplied")
	}

	overwrite := args.GetBool(WriteFileOverwrite)

	_, statErr := os.Stat(fullPath)
	if statErr != nil && !errors.Is(statErr, fs.ErrNotExist) {
		return "", fmt.Errorf("failed to stat %q: %w", fullPath, statErr)
	}

	if statErr == nil && (overwrite == nil || !*overwrite) {
		return "", fmt.Errorf("refusing to overwrite %q", fullPath)
	}

	if err := os.WriteFile(fullPath, []byte(*contents), 0o644); err != nil {
		return "", fmt.Errorf("failed to write contents to %q: %w", fullPath, err)
	}

	return fmt.Sprintf("Wrote contents to %q", *path), nil
}
