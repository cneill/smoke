package mkdir

import (
	"context"
	"fmt"
	"os"

	"github.com/cneill/smoke/pkg/fs"
	"github.com/cneill/smoke/pkg/tools"
)

const (
	Name      = "mkdir"
	ParamPath = "path"
)

type Mkdir struct {
	ProjectPath string
}

func New(projectPath, _ string) tools.Tool {
	return &Mkdir{
		ProjectPath: projectPath,
	}
}

func (m *Mkdir) Name() string { return Name }
func (m *Mkdir) Description() string {
	examples := tools.CollectExamples(m.Examples()...)

	return fmt.Sprintf("Create a new directory at %q. Will create intermediate directories if necessary.%s",
		ParamPath, examples)
}

func (m *Mkdir) Examples() tools.Examples {
	return tools.Examples{
		{
			Description: `Create the new directory "pkg/new_pkg" inside the project directory`,
			Args: tools.Args{
				ParamPath: "pkg/new_pkg",
			},
		},
	}
}

func (m *Mkdir) Params() tools.Params {
	return tools.Params{
		{
			Key:         ParamPath,
			Description: "The path where the directory should be created",
			Type:        tools.ParamTypeString,
			Required:    true,
		},
	}
}

func (m *Mkdir) Run(_ context.Context, args tools.Args) (string, error) {
	path := args.GetString(ParamPath)
	if path == nil {
		return "", fmt.Errorf("%w: no path supplied", tools.ErrArguments)
	}

	fullPath, err := fs.GetRelativePath(m.ProjectPath, *path)
	if err != nil {
		return "", fmt.Errorf("%w: path error: %w", err, tools.ErrArguments)
	}

	err = os.MkdirAll(fullPath, 0o755)
	if err != nil {
		return "", fmt.Errorf("%w: failed to create directory %q: %w", tools.ErrFileSystem, fullPath, err)
	}

	return fmt.Sprintf("Created directory at %q", *path), nil
}
