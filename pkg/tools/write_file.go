package tools

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"

	smokefs "github.com/cneill/smoke/pkg/fs"
)

const (
	WriteFilePath     = "path"
	WriteFileContents = "contents"
)

type WriteFileTool struct {
	ProjectPath string
}

func NewWriteFileTool(projectPath, _ string) Tool {
	return &WriteFileTool{ProjectPath: projectPath}
}

func (w *WriteFileTool) Name() string { return ToolWriteFile }
func (w *WriteFileTool) Description() string {
	examples := CollectExamples(w.Examples()...)

	return fmt.Sprintf("Create a new file at the path specified in %q and write the contents in %q to it. Cannot edit "+
		"existing files.%s", WriteFilePath, WriteFileContents, examples,
	)
}

func (w *WriteFileTool) Examples() Examples {
	return Examples{
		{
			Description: `Create the file "cmd/tool/main.go" and write a simple program to it`,
			Args: Args{
				WriteFilePath:     "cmd/tool/main.go",
				WriteFileContents: "package main\n\nfunc main() {\n\tfmt.Printf(\"Hello, world\")\n}\n",
			},
		},
	}
}

func (w *WriteFileTool) Params() Params {
	return Params{
		{
			Key:         WriteFilePath,
			Description: "The path of the file to be created and written to",
			Type:        ParamTypeString,
			Required:    true,
		},
		{
			Key:         WriteFileContents,
			Description: "The contents to write to the new file",
			Type:        ParamTypeString,
			Required:    true,
		},
	}
}

func (w *WriteFileTool) Run(_ context.Context, args Args) (string, error) {
	path := args.GetString(WriteFilePath)
	if path == nil {
		return "", fmt.Errorf("%w: no path supplied", ErrArguments)
	}

	fullPath, err := smokefs.GetRelativePath(w.ProjectPath, *path)
	if err != nil {
		return "", fmt.Errorf("%w: path error: %w", ErrArguments, err)
	}

	contents := args.GetString(WriteFileContents)
	if contents == nil {
		return "", fmt.Errorf("%w: no contents supplied", ErrArguments)
	}

	_, statErr := os.Stat(fullPath)
	if statErr != nil && !errors.Is(statErr, fs.ErrNotExist) {
		return "", fmt.Errorf("%w: failed to stat %q: %w", ErrFileSystem, *path, statErr)
	}

	if statErr == nil {
		return "", fmt.Errorf("%w: refusing to overwrite %q", ErrFileSystem, *path)
	}

	if err := os.WriteFile(fullPath, []byte(*contents), 0o644); err != nil {
		return "", fmt.Errorf("%w: failed to write contents to %q: %w", ErrFileSystem, *path, err)
	}

	return fmt.Sprintf("Wrote contents to %q", *path), nil
}
