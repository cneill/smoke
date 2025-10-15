package writefile

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"

	smokefs "github.com/cneill/smoke/pkg/fs"
	"github.com/cneill/smoke/pkg/tools"
)

const (
	Name          = "writefile"
	ParamPath     = "path"
	ParamContents = "contents"
)

type WriteFile struct {
	ProjectPath string
}

func New(projectPath, _ string) tools.Tool {
	return &WriteFile{ProjectPath: projectPath}
}

func (w *WriteFile) Name() string { return Name }
func (w *WriteFile) Description() string {
	examples := tools.CollectExamples(w.Examples()...)

	return fmt.Sprintf("Create a new file at the path specified in %q and write the contents in %q to it. Cannot edit "+
		"existing files.%s", ParamPath, ParamContents, examples,
	)
}

func (w *WriteFile) Examples() tools.Examples {
	return tools.Examples{
		{
			Description: `Create the file "cmd/tool/main.go" and write a simple program to it`,
			Args: tools.Args{
				ParamPath:     "cmd/tool/main.go",
				ParamContents: "package main\n\nfunc main() {\n\tfmt.Printf(\"Hello, world\")\n}\n",
			},
		},
	}
}

func (w *WriteFile) Params() tools.Params {
	return tools.Params{
		{
			Key:         ParamPath,
			Description: "The path of the file to be created and written to",
			Type:        tools.ParamTypeString,
			Required:    true,
		},
		{
			Key:         ParamContents,
			Description: "The contents to write to the new file",
			Type:        tools.ParamTypeString,
			Required:    true,
		},
	}
}

func (w *WriteFile) Run(_ context.Context, args tools.Args) (string, error) {
	path := args.GetString(ParamPath)
	if path == nil {
		return "", fmt.Errorf("%w: no path supplied", tools.ErrArguments)
	}

	fullPath, err := smokefs.GetRelativePath(w.ProjectPath, *path)
	if err != nil {
		return "", fmt.Errorf("%w: path error: %w", tools.ErrArguments, err)
	}

	contents := args.GetString(ParamContents)
	if contents == nil {
		return "", fmt.Errorf("%w: no contents supplied", tools.ErrArguments)
	}

	_, statErr := os.Stat(fullPath)
	if statErr != nil && !errors.Is(statErr, fs.ErrNotExist) {
		return "", fmt.Errorf("%w: failed to stat %q: %w", tools.ErrFileSystem, *path, statErr)
	}

	if statErr == nil {
		return "", fmt.Errorf("%w: refusing to overwrite %q", tools.ErrFileSystem, *path)
	}

	if err := os.WriteFile(fullPath, []byte(*contents), 0o644); err != nil {
		return "", fmt.Errorf("%w: failed to write contents to %q: %w", tools.ErrFileSystem, *path, err)
	}

	return fmt.Sprintf("Wrote contents to %q", *path), nil
}
