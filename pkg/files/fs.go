package files

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/cneill/smoke/pkg/files/ignores"
	"github.com/cneill/smoke/pkg/files/paths"
)

var ErrIgnored = errors.New("ignored file")

type Opts struct {
	ConfigDir  string
	ProjectDir string
}

func (o Opts) OK() error {
	switch {
	case o.ConfigDir == "":
		return fmt.Errorf("missing config dir")
	case o.ProjectDir == "":
		return fmt.Errorf("missing project dir")
	case !filepath.IsAbs(o.ConfigDir):
		return fmt.Errorf("config dir %q is not absolute", o.ConfigDir)
	case !filepath.IsAbs(o.ProjectDir):
		return fmt.Errorf("project dir %q is not absolute", o.ProjectDir)
	}

	if err := paths.RequiredDir(o.ProjectDir); err != nil {
		return err
	}

	return nil
}

type ProjectFS struct {
	root *os.Root

	configDir  string
	projectDir string
	ignorer    *ignores.Ignorer
}

func NewProjectFS(opts Opts) (*ProjectFS, error) {
	if !filepath.IsAbs(opts.ProjectDir) {
		return nil, fmt.Errorf("project path %q is not absolute", opts.ProjectDir)
	}

	root, err := os.OpenRoot(opts.ProjectDir)
	if err != nil {
		return nil, fmt.Errorf("failed to open project path as a root: %w", err)
	}

	ignorer, err := ignores.NewIgnorer(ignores.Opts{
		ConfigDir:  opts.ConfigDir,
		ProjectDir: opts.ProjectDir,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to set up ignored file handling: %w", err)
	}

	pfs := &ProjectFS{
		root: root,

		configDir:  opts.ConfigDir,
		projectDir: opts.ProjectDir,
		ignorer:    ignorer,
	}

	return pfs, nil
}

func (p *ProjectFS) ProjectDir() string { return p.projectDir }

func (p *ProjectFS) Stat(name string) (fs.FileInfo, error) {
	fullPath, err := p.FullPath(name)
	if err != nil {
		return nil, fmt.Errorf("invalid path: %w", err)
	}

	fileInfo, err := p.root.Stat(name)
	if err != nil {
		return nil, fmt.Errorf("failed to stat: %w", err)
	}

	if ignored, _ := p.ignorer.Ignored(fullPath, fileInfo.IsDir()); ignored {
		return nil, ErrIgnored
	}

	return fileInfo, nil
}

func (p *ProjectFS) Open(name string) (*os.File, error) {
	fullPath, err := p.FullPath(name)
	if err != nil {
		return nil, fmt.Errorf("invalid path: %w", err)
	}

	fileInfo, err := p.root.Stat(name)
	if err != nil {
		return nil, fmt.Errorf("failed to stat: %w", err)
	}

	if ignored, _ := p.ignorer.Ignored(fullPath, fileInfo.IsDir()); ignored {
		return nil, ErrIgnored
	}

	return p.root.Open(name)
}

func (p *ProjectFS) ReadFile(name string) ([]byte, error) {
	f, err := p.Open(name)
	if err != nil {
		return nil, err
	}

	defer f.Close()

	return io.ReadAll(f)
}

func (p *ProjectFS) ReadDir(name string) ([]fs.DirEntry, error) {
	rdFS, ok := p.root.FS().(fs.ReadDirFS)
	if !ok {
		return nil, fmt.Errorf("root file system set up improperly, can't read directory")
	}

	initEntries, err := rdFS.ReadDir(name)
	if err != nil {
		return nil, fmt.Errorf("failed to read dir: %w", err)
	}

	entries := make([]fs.DirEntry, 0, len(initEntries))

	for _, entry := range initEntries {
		relPath := filepath.Join(name, entry.Name())
		fullPath, err := p.FullPath(relPath)
		slog.Error("invalid path while reading dir", "path", relPath, "error", err)

		if ignored, _ := p.ignorer.Ignored(fullPath, entry.IsDir()); ignored {
			continue
		}

		entries = append(entries, entry)
	}

	return entries, nil
}

func (p *ProjectFS) FullPath(name string) (string, error) {
	if filepath.IsAbs(name) {
		return "", fmt.Errorf("must supply relative path")
	}

	if strings.HasPrefix(name, "..") ||
		strings.Contains(name, "../") ||
		strings.Contains(name, "..\\") ||
		strings.Contains(name, "/..") ||
		strings.Contains(name, "\\..") {
		return "", fmt.Errorf("path contains traversal")
	}

	return filepath.Join(p.projectDir, filepath.Clean(name)), nil
}
