package files_test

import (
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/cneill/smoke/pkg/files"
	"github.com/cneill/smoke/pkg/files/ignores"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type projectFSTestEnv struct {
	configDir  string
	projectDir string
}

func setupProjectFSEnv(t *testing.T) projectFSTestEnv {
	t.Helper()

	rootDir := t.TempDir()
	env := projectFSTestEnv{
		configDir:  filepath.Join(rootDir, "config"),
		projectDir: filepath.Join(rootDir, "project"),
	}

	require.NoError(t, os.MkdirAll(env.configDir, 0o755))
	require.NoError(t, os.MkdirAll(env.projectDir, 0o755))

	return env
}

func writeProjectFile(t *testing.T, rootDir, relPath, contents string) {
	t.Helper()

	fullPath := filepath.Join(rootDir, relPath)
	require.NoError(t, os.MkdirAll(filepath.Dir(fullPath), 0o755))
	require.NoError(t, os.WriteFile(fullPath, []byte(contents), 0o644))
}

func mkdirProjectDir(t *testing.T, rootDir, relPath string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Join(rootDir, relPath), 0o755))
}

func newProjectFS(t *testing.T, env projectFSTestEnv) *files.ProjectFS {
	t.Helper()

	projectFS, err := files.NewProjectFS(files.Opts{
		ConfigDir:  env.configDir,
		ProjectDir: env.projectDir,
	})
	require.NoError(t, err)

	return projectFS
}

const (
	pathVisibleFile      = "visible.txt"
	pathMissingFile      = "missing.txt"
	pathIgnoredLog       = "app.log"
	pathTraversalFile    = "../secret.txt"
	errPathTraversal     = "path contains traversal"
	pathVisibleDirectory = "nested/dir"
)

func entryNames(entries []fs.DirEntry) []string {
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		names = append(names, entry.Name())
	}

	sort.Strings(names)

	return names
}

func TestProjectFSFullPath(t *testing.T) {
	t.Parallel()

	env := setupProjectFSEnv(t)
	projectFS := newProjectFS(t, env)

	tests := []struct {
		name        string
		path        string
		expected    string
		expectedErr string
	}{
		{
			name:     "simple relative path resolves under project dir",
			path:     "cmd/main.go",
			expected: filepath.Join(env.projectDir, "cmd", "main.go"),
		},
		{
			name:     "dot segments are cleaned",
			path:     "pkg/./files/fs.go",
			expected: filepath.Join(env.projectDir, "pkg", "files", "fs.go"),
		},
		{
			name:        "absolute path is rejected",
			path:        filepath.Join(env.projectDir, "cmd", "main.go"),
			expectedErr: "must supply relative path",
		},
		{
			name:        "parent traversal prefix is rejected",
			path:        pathTraversalFile,
			expectedErr: errPathTraversal,
		},
		{
			name:        "nested unix traversal is rejected",
			path:        "cmd/../../secret.txt",
			expectedErr: errPathTraversal,
		},
		{
			name:        "windows traversal is rejected",
			path:        "cmd\\..\\secret.txt",
			expectedErr: errPathTraversal,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			fullPath, err := projectFS.FullPath(test.path)
			if test.expectedErr != "" {
				require.EqualError(t, err, test.expectedErr)
				assert.Empty(t, fullPath)

				return
			}

			require.NoError(t, err)
			assert.Equal(t, test.expected, fullPath)
		})
	}
}

func TestProjectFSOpenReadFileAndStat(t *testing.T) { //nolint:funlen
	t.Parallel()

	tests := []struct {
		name          string
		configIgnore  string
		projectIgnore string
		prepare       func(t *testing.T, env projectFSTestEnv)
		openPath      string
		statPath      string
		readFilePath  string
		assertResult  func(t *testing.T, openErr error, fileInfo fs.FileInfo, statErr error, contents []byte, readErr error)
	}{
		{
			name: "visible file succeeds",
			prepare: func(t *testing.T, env projectFSTestEnv) {
				t.Helper()
				writeProjectFile(t, env.projectDir, pathVisibleFile, "hello world")
			},
			openPath:     pathVisibleFile,
			statPath:     pathVisibleFile,
			readFilePath: pathVisibleFile,
			assertResult: func(t *testing.T, openErr error, fileInfo fs.FileInfo, statErr error, contents []byte, readErr error) {
				t.Helper()
				require.NoError(t, openErr)
				require.NoError(t, statErr)
				require.NoError(t, readErr)
				require.NotNil(t, fileInfo)
				assert.False(t, fileInfo.IsDir())
				assert.Equal(t, "hello world", string(contents))
			},
		},
		{
			name: "visible directory stats successfully",
			prepare: func(t *testing.T, env projectFSTestEnv) {
				t.Helper()
				mkdirProjectDir(t, env.projectDir, pathVisibleDirectory)
			},
			statPath: pathVisibleDirectory,
			assertResult: func(t *testing.T, openErr error, fileInfo fs.FileInfo, statErr error, contents []byte, readErr error) {
				t.Helper()
				require.NoError(t, openErr)
				require.NoError(t, statErr)
				require.NoError(t, readErr)
				assert.Empty(t, contents)
				require.NotNil(t, fileInfo)
				assert.True(t, fileInfo.IsDir())
			},
		},
		{
			name:         "missing path returns wrapped stat errors",
			openPath:     pathMissingFile,
			statPath:     pathMissingFile,
			readFilePath: pathMissingFile,
			assertResult: func(t *testing.T, openErr error, fileInfo fs.FileInfo, statErr error, contents []byte, readErr error) {
				t.Helper()
				require.ErrorContains(t, openErr, "failed to stat")
				require.ErrorContains(t, statErr, "failed to stat")
				require.ErrorContains(t, readErr, "failed to stat")
				assert.Nil(t, fileInfo)
				assert.Empty(t, contents)
			},
		},
		{
			name:         "invalid path returns invalid path errors",
			openPath:     pathTraversalFile,
			statPath:     pathTraversalFile,
			readFilePath: pathTraversalFile,
			assertResult: func(t *testing.T, openErr error, fileInfo fs.FileInfo, statErr error, contents []byte, readErr error) {
				t.Helper()
				require.ErrorContains(t, openErr, "invalid path")
				require.ErrorContains(t, statErr, "invalid path")
				require.ErrorContains(t, readErr, "invalid path")
				assert.Nil(t, fileInfo)
				assert.Empty(t, contents)
			},
		},
		{
			name:         "config ignored file returns ErrIgnored",
			configIgnore: "*.log\n",
			prepare: func(t *testing.T, env projectFSTestEnv) {
				t.Helper()
				writeProjectFile(t, env.projectDir, pathIgnoredLog, "ignored")
			},
			openPath:     pathIgnoredLog,
			statPath:     pathIgnoredLog,
			readFilePath: pathIgnoredLog,
			assertResult: func(t *testing.T, openErr error, fileInfo fs.FileInfo, statErr error, contents []byte, readErr error) {
				t.Helper()
				require.ErrorIs(t, openErr, files.ErrIgnored)
				require.ErrorIs(t, statErr, files.ErrIgnored)
				require.ErrorIs(t, readErr, files.ErrIgnored)
				assert.Nil(t, fileInfo)
				assert.Empty(t, contents)
			},
		},
		{
			name:          "project ignored directory returns ErrIgnored",
			projectIgnore: "dist/\n",
			prepare: func(t *testing.T, env projectFSTestEnv) {
				t.Helper()
				mkdirProjectDir(t, env.projectDir, "dist")
			},
			statPath: "dist",
			assertResult: func(t *testing.T, openErr error, fileInfo fs.FileInfo, statErr error, contents []byte, readErr error) {
				t.Helper()
				require.NoError(t, openErr)
				require.ErrorIs(t, statErr, files.ErrIgnored)
				require.NoError(t, readErr)
				assert.Nil(t, fileInfo)
				assert.Empty(t, contents)
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			env := setupProjectFSEnv(t)
			if test.configIgnore != "" {
				writeProjectFile(t, env.configDir, ignores.IgnoreName, test.configIgnore)
			}

			if test.projectIgnore != "" {
				writeProjectFile(t, env.projectDir, ignores.IgnoreDotName, test.projectIgnore)
			}

			if test.prepare != nil {
				test.prepare(t, env)
			}

			projectFS := newProjectFS(t, env)

			var (
				openedFile *os.File
				openErr    error
			)
			if test.openPath != "" {
				openedFile, openErr = projectFS.Open(test.openPath)
				if openedFile != nil {
					defer openedFile.Close()
				}
			}

			var (
				fileInfo fs.FileInfo
				statErr  error
			)
			if test.statPath != "" {
				fileInfo, statErr = projectFS.Stat(test.statPath)
			}

			var (
				contents []byte
				readErr  error
			)
			if test.readFilePath != "" {
				contents, readErr = projectFS.ReadFile(test.readFilePath)
			}

			test.assertResult(t, openErr, fileInfo, statErr, contents, readErr)
		})
	}
}

func TestProjectFSReadDirFiltersIgnoredEntries(t *testing.T) {
	t.Parallel()

	env := setupProjectFSEnv(t)
	writeProjectFile(t, env.configDir, ignores.IgnoreName, "*.log\n")
	writeProjectFile(t, env.projectDir, ignores.IgnoreDotName, "dist/\n")
	writeProjectFile(t, env.projectDir, "visible.txt", "visible")
	writeProjectFile(t, env.projectDir, "debug.log", "ignored by config")
	mkdirProjectDir(t, env.projectDir, "docs")
	mkdirProjectDir(t, env.projectDir, "dist")
	writeProjectFile(t, env.projectDir, "docs/readme.md", "visible nested")
	writeProjectFile(t, env.projectDir, "dist/app.js", "ignored nested")

	projectFS := newProjectFS(t, env)

	entries, err := projectFS.ReadDir(".")
	require.NoError(t, err)
	assert.Equal(t, []string{".smokeignore", "docs", "visible.txt"}, entryNames(entries))

	docEntries, err := projectFS.ReadDir("docs")
	require.NoError(t, err)
	assert.Equal(t, []string{"readme.md"}, entryNames(docEntries))
}
