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
		path            string
		isDir           bool
		expected        bool
	}{
		{
			name:     "no ignore files means nothing is ignored",
			path:     "tmp/cache.log",
			expected: false,
		},
		{
			name:           "config ignore file is discovered and parsed",
			configContents: "\n# comment\n*.log\n",
			path:           "tmp/cache.log",
			expected:       true,
		},
		{
			name:            "project ignore file is discovered but does not match relative paths",
			projectContents: "\n# comment\nbuild/\n",
			path:            "build",
			isDir:           true,
			expected:        false,
		},
		{
			name:            "both ignore files are discovered",
			configContents:  logPattern,
			projectContents: "build/\n",
			path:            "build/output.log",
			expected:        true,
		},
		{
			name:           "blank lines and comments are ignored during parsing",
			configContents: "\n   \n# comment\n\n*.tmp\n",
			path:           "scratch.tmp",
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

			assert.Equal(t, test.expected, ignorer.Ignored(test.path, test.isDir))
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
			name:            "project patterns do not match project relative paths with current path scoping",
			projectContents: distPattern,
			path:            "dist",
			isDir:           true,
			expected:        false,
		},
		{
			name:            "project file patterns do not match relative descendants with current path scoping",
			projectContents: distPattern,
			path:            "dist/output/app.js",
			expected:        false,
		},
		{
			name:            "project scoped basename patterns do not match relative project paths",
			projectContents: "vendor/\n",
			path:            "pkg/vendor/lib.go",
			expected:        false,
		},
		{
			name:            "project scoped file patterns do not match relative nested project files",
			projectContents: "*.local\n",
			path:            "config/dev.local",
			expected:        false,
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

			assert.Equal(t, test.expected, ignorer.Ignored(test.path, test.isDir))
		})
	}
}
