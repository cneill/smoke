package ignores_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/cneill/smoke/pkg/files/ignores"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	logPattern  = "*.log\n"
	distPattern = "dist/\n"
)

type ignoresTestEnv struct {
	configDir         string
	projectDir        string
	configIgnorePath  string
	projectIgnorePath string
}

func setupIgnoresEnv(t *testing.T) ignoresTestEnv {
	t.Helper()

	rootDir := t.TempDir()
	configDir := filepath.Join(rootDir, "config")
	projectDir := filepath.Join(rootDir, "project")

	require.NoError(t, os.MkdirAll(configDir, 0o755))
	require.NoError(t, os.MkdirAll(projectDir, 0o755))

	return ignoresTestEnv{
		configDir:         configDir,
		projectDir:        projectDir,
		configIgnorePath:  filepath.Join(configDir, ignores.IgnoreName),
		projectIgnorePath: filepath.Join(projectDir, ignores.IgnoreDotName),
	}
}

func writeFile(t *testing.T, path, contents string) {
	t.Helper()

	require.NoError(t, os.WriteFile(path, []byte(contents), 0o644))
}

func newIgnorer(t *testing.T, env ignoresTestEnv) *ignores.Ignorer {
	t.Helper()

	ignorer, err := ignores.NewIgnorer(ignores.Opts{
		ConfigDir:  env.configDir,
		ProjectDir: env.projectDir,
	})
	require.NoError(t, err)

	return ignorer
}

func TestIgnorerFiles(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		configContents  string
		projectContents string
		relPath         string
		isDir           bool
		expected        bool
	}{
		{
			name:     "no ignore files means nothing is ignored",
			relPath:  "tmp/cache.log",
			expected: false,
		},
		{
			name:           "config ignore file is discovered and parsed",
			configContents: "\n# comment\n*.log\n",
			relPath:        "tmp/cache.log",
			expected:       true,
		},
		{
			name:            "project ignore file matches absolute path under project dir",
			projectContents: "\n# comment\nbuild/\n",
			relPath:         "build",
			isDir:           true,
			expected:        true,
		},
		{
			name:            "both ignore files are discovered",
			configContents:  logPattern,
			projectContents: "build/\n",
			relPath:         "build/output.log",
			expected:        true,
		},
		{
			name:           "blank lines and comments are ignored during parsing",
			configContents: "\n   \n# comment\n\n*.tmp\n",
			relPath:        "scratch.tmp",
			expected:       true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			env := setupIgnoresEnv(t)

			if test.configContents != "" {
				writeFile(t, env.configIgnorePath, test.configContents)
			}

			if test.projectContents != "" {
				writeFile(t, env.projectIgnorePath, test.projectContents)
			}

			ignorer := newIgnorer(t, env)
			absPath := filepath.Join(env.projectDir, test.relPath)

			ignored, err := ignorer.Ignored(absPath, test.isDir)
			require.NoError(t, err)
			assert.Equal(t, test.expected, ignored)
		})
	}
}

func TestIgnorerIgnored(t *testing.T) { //nolint:funlen
	t.Parallel()

	tests := []struct {
		name            string
		configContents  string
		projectContents string
		path            string
		isDir           bool
		expected        bool
		expectedErr     string
	}{
		{
			name:           "config patterns apply globally across directories",
			configContents: logPattern,
			path:           "nested/app.log",
			expected:       true,
		},
		{
			name:           "config patterns do not match different names",
			configContents: logPattern,
			path:           "nested/app.txt",
			expected:       false,
		},
		{
			name:            "project directory pattern matches absolute path under project dir",
			projectContents: distPattern,
			path:            "dist",
			isDir:           true,
			expected:        true,
		},
		{
			name:            "project directory pattern matches absolute descendant paths",
			projectContents: distPattern,
			path:            "dist/output/app.js",
			expected:        true,
		},
		{
			name:            "project scoped basename pattern matches nested directories",
			projectContents: "vendor/\n",
			path:            "pkg/vendor/lib.go",
			expected:        true,
		},
		{
			name:            "project scoped glob pattern matches nested files",
			projectContents: "*.local\n",
			path:            "config/dev.local",
			expected:        true,
		},
		{
			name:            "config patterns still match when project patterns are also loaded",
			configContents:  logPattern,
			projectContents: distPattern,
			path:            "dist/server.log",
			expected:        true,
		},
		{
			name:            "non matching path remains visible",
			configContents:  logPattern,
			projectContents: distPattern,
			path:            "cmd/server.go",
			expected:        false,
		},
		{
			name:           "nested path remains visible",
			configContents: "dist/output/*.js",
			path:           "app.js",
			expected:       false,
		},
		{
			name:            "relative path returns an error",
			configContents:  logPattern,
			projectContents: distPattern,
			path:            "dist/server.log",
			expected:        false,
			expectedErr:     "path \"dist/server.log\" is not absolute",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			env := setupIgnoresEnv(t)

			if test.configContents != "" {
				writeFile(t, env.configIgnorePath, test.configContents)
			}

			if test.projectContents != "" {
				writeFile(t, env.projectIgnorePath, test.projectContents)
			}

			ignorer := newIgnorer(t, env)

			path := test.path
			if test.expectedErr == "" {
				path = filepath.Join(env.projectDir, test.path)
			}

			ignored, err := ignorer.Ignored(path, test.isDir)
			assert.Equal(t, test.expected, ignored)

			if test.expectedErr != "" {
				require.EqualError(t, err, test.expectedErr)
				return
			}

			require.NoError(t, err)
		})
	}
}
