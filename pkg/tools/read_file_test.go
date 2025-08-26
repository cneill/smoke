package tools_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cneill/smoke/pkg/tools"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReadFileTool_Run(t *testing.T) { //nolint:funlen
	t.Parallel()

	tempDir := t.TempDir()
	readFileTool := &tools.ReadFileTool{ProjectPath: tempDir}

	tests := []struct {
		name           string
		initContent    string
		args           tools.Args
		expectedOutput string
		err            error
	}{
		{
			name:           "nil_args",
			initContent:    "test",
			args:           nil,
			expectedOutput: "",
			err:            tools.ErrArguments,
		},
		{
			name:        "empty_path",
			initContent: "test",
			args: tools.Args{
				tools.ReadFileStartLine: 1,
				tools.ReadFileEndLine:   2,
			},
			expectedOutput: "",
			err:            tools.ErrArguments,
		},
		{
			name:        "invalid_path",
			initContent: "test",
			args: tools.Args{
				tools.ReadFilePath: "garbage.txt",
			},
			expectedOutput: "",
			err:            tools.ErrFileSystem,
		},
		{
			name:        "invalid_start",
			initContent: "test",
			args: tools.Args{
				tools.ReadFilePath:      "invalid_start_test.txt",
				tools.ReadFileStartLine: -1,
			},
			expectedOutput: "",
			err:            tools.ErrArguments,
		},
		{
			name:        "invalid_end",
			initContent: "test",
			args: tools.Args{
				tools.ReadFilePath:    "invalid_end_test.txt",
				tools.ReadFileEndLine: -1,
			},
			expectedOutput: "",
			err:            tools.ErrArguments,
		},
		{
			name:        "end_before_start",
			initContent: "test",
			args: tools.Args{
				tools.ReadFilePath:      "end_before_start.txt",
				tools.ReadFileStartLine: 2,
				tools.ReadFileEndLine:   1,
			},
			expectedOutput: "",
			err:            tools.ErrArguments,
		},
		{
			name:        "start_beyond_eof",
			initContent: "test1\ntest2\n",
			args: tools.Args{
				tools.ReadFilePath:      "start_beyond_eof_test.txt",
				tools.ReadFileStartLine: 4,
				tools.ReadFileEndLine:   6,
			},
			expectedOutput: "",
			err:            tools.ErrArguments,
		},
		{
			name:        "end_at_eof",
			initContent: "test1\ntest2",
			args: tools.Args{
				tools.ReadFilePath:      "end_at_eof_test.txt",
				tools.ReadFileStartLine: 1,
				tools.ReadFileEndLine:   2,
			},
			expectedOutput: "1: test1\n2: test2\n",
			err:            nil,
		},
		{
			name:        "end_beyond_eof",
			initContent: "test1\ntest2",
			args: tools.Args{
				tools.ReadFilePath:      "end_beyond_eof_test.txt",
				tools.ReadFileStartLine: 1,
				tools.ReadFileEndLine:   4,
			},
			expectedOutput: "1: test1\n2: test2\n",
			err:            nil,
		},
		{
			name:        "binary_content",
			initContent: "\x00\x01\x02\x03\x00",
			args: tools.Args{
				tools.ReadFilePath: "binary_content_test.txt",
			},
			expectedOutput: "[binary content]",
			err:            nil,
		},
		{
			name:        "full_file",
			initContent: "test1\ntest2",
			args: tools.Args{
				tools.ReadFilePath: "full_file_test.txt",
			},
			expectedOutput: "1: test1\n2: test2\n",
			err:            nil,
		},
		{
			name:        "single_line",
			initContent: "test1\ntest2\ntest3",
			args: tools.Args{
				tools.ReadFilePath:      "single_line_test.txt",
				tools.ReadFileStartLine: 2,
				tools.ReadFileEndLine:   2,
			},
			expectedOutput: "2: test2\n",
			err:            nil,
		},
		{
			name:        "line_num_width",
			initContent: strings.Repeat("test\n", 20),
			args: tools.Args{
				tools.ReadFilePath:      "line_num_width_test.txt",
				tools.ReadFileStartLine: 9,
				tools.ReadFileEndLine:   11,
			},
			expectedOutput: " 9: test\n10: test\n11: test\n",
			err:            nil,
		},
		{
			name:        "no_end",
			initContent: "test1\ntest2\ntest3\n",
			args: tools.Args{
				tools.ReadFilePath:      "no_end_test.txt",
				tools.ReadFileStartLine: 2,
			},
			expectedOutput: "2: test2\n3: test3\n",
			err:            nil,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			fileName := test.name + "_test.txt"
			tempPath := filepath.Join(tempDir, fileName)

			tempFile, err := os.Create(tempPath)
			require.NoError(t, err, "failed to create temporary file: %w", err)

			defer tempFile.Close()

			_, writeErr := tempFile.WriteString(test.initContent)
			require.NoError(t, writeErr, "failed to write initial content to file %q: %v", tempPath, writeErr)

			output, runErr := readFileTool.Run(t.Context(), test.args)
			if test.err == nil {
				require.NoError(t, runErr, "unexpected error: %v", runErr)
			} else {
				require.ErrorIs(t, runErr, test.err)
			}

			assert.Equal(t, test.expectedOutput, output)
		})
	}
}
