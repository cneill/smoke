package fs_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/cneill/smoke/pkg/fs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestListPathMatches(t *testing.T) { //nolint:funlen
	t.Parallel()

	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, "docs", "api"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(root, "pkg", "models"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, "docs", "readme.md"), []byte("x"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(root, "docs", "api", "openapi.yaml"), []byte("x"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(root, "main.go"), []byte("x"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(root, "Makefile"), []byte("x"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(root, ".hidden"), []byte("x"), 0o644))

	// smokeignore should exclude matching names under the project
	require.NoError(t, os.WriteFile(filepath.Join(root, ".smokeignore"), []byte(".hidden\n"), 0o644))

	tests := []struct {
		name     string
		query    string
		expected []string
	}{
		{
			name:     "empty_query_no_matches",
			query:    "",
			expected: []string{},
		},
		{
			name:  "prefix_at_root",
			query: "do",
			expected: []string{
				"docs/",
			},
		},
		{
			name:  "exact_dir_prefix_then_list_children",
			query: "docs/",
			expected: []string{
				"docs/api/",
				"docs/readme.md",
			},
		},
		{
			name:  "nested_prefix",
			query: "docs/r",
			expected: []string{
				"docs/readme.md",
			},
		},
		{
			name:  "file_prefix_at_root",
			query: "ma",
			expected: []string{
				"main.go",
			},
		},
		{
			name:     "no_match",
			query:    "zzz",
			expected: []string{},
		},
		{
			name:     "traversal_rejected",
			query:    "../",
			expected: []string{},
		},
		{
			name:     "ignored_hidden_not_listed",
			query:    ".h",
			expected: []string{},
		},
		{
			name:  "multiple_prefix_matches_sorted",
			query: "M",
			expected: []string{
				"Makefile",
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			matches, err := fs.ListPathMatches(root, test.query)
			require.NoError(t, err)

			got := make([]string, len(matches))
			for i, m := range matches {
				got[i] = m.Path
			}

			assert.Equal(t, test.expected, got)
		})
	}
}

func TestCompleter(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, "pkg"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, "pkg", "a.go"), []byte("x"), 0o644))

	complete := fs.PathCompleter(root)
	matches := complete("pkg/")
	require.Len(t, matches, 1)
	assert.Equal(t, "pkg/a.go", matches[0].Path)
	assert.False(t, matches[0].IsDir)

	// insecure / bad project path yields empty, not panic
	assert.Nil(t, fs.PathCompleter("")("pkg"))
	assert.Nil(t, fs.PathCompleter("relative")("pkg"))
}
