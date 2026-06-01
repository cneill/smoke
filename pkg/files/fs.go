package files

import (
	"fmt"
	"os"
	"path/filepath"
)

type ProjectFS struct {
	projectPath string
	root        *os.Root
}

func NewProjectFS(projectPath string) (*ProjectFS, error) {
	if !filepath.IsAbs(projectPath) {
		return nil, fmt.Errorf("project path %q is not absolute", projectPath)
	}

	root, err := os.OpenRoot(projectPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open project path as a root: %w", err)
	}

	pfs := &ProjectFS{
		projectPath: projectPath,
		root:        root,
	}

	return pfs, nil
}

func (p *ProjectFS) ProjectPath() string { return p.projectPath }
