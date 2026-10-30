package smoke //nolint:testpackage

import (
	"os"
	"path/filepath"
	"testing"

	git "github.com/go-git/go-git/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWithProjectPath(t *testing.T) {
	t.Parallel()

	// WithProjectPath resolves symlinks, and temp dirs may contain them (e.g. /var -> /private/var on macOS), so pass
	// the raw paths as inputs and compare against resolved paths.
	resolve := func(path string) string {
		resolved, err := filepath.EvalSymlinks(path)
		require.NoError(t, err)

		return resolved
	}

	repoDir := t.TempDir()
	_, err := git.PlainInit(repoDir, false)
	require.NoError(t, err)

	subDir := filepath.Join(repoDir, "a", "b")
	require.NoError(t, os.MkdirAll(subDir, 0o755))

	repoLink := filepath.Join(t.TempDir(), "repo-link")
	require.NoError(t, os.Symlink(repoDir, repoLink))

	nonRepoDir := t.TempDir()

	tests := []struct {
		name     string
		path     string
		noGit    bool
		wantPath string
		wantErr  bool
	}{
		{name: "repo root", path: repoDir, wantPath: resolve(repoDir)},
		{name: "repo subdirectory resolves to root", path: subDir, wantPath: resolve(repoDir)},
		{name: "symlink to repo resolves", path: repoLink, wantPath: resolve(repoDir)},
		{name: "symlink to repo with noGit resolves", path: repoLink, noGit: true, wantPath: resolve(repoDir)},
		{name: "not a repo", path: nonRepoDir, wantErr: true},
		{name: "not a repo with noGit", path: nonRepoDir, noGit: true, wantPath: resolve(nonRepoDir)},
		{name: "repo subdirectory with noGit is kept", path: subDir, noGit: true, wantPath: resolve(subDir)},
		{name: "nonexistent path", path: filepath.Join(nonRepoDir, "missing"), noGit: true, wantErr: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			smoke, err := WithProjectPath(test.path, test.noGit)(&Smoke{})
			if test.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, test.wantPath, smoke.projectPath)
		})
	}
}
