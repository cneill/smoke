package tools_test

import (
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/cneill/smoke/pkg/tools"
	"github.com/cneill/smoke/pkg/utils"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TODO: check the output as well
func TestReplaceLinesV2Tool_Run(t *testing.T) { //nolint:funlen
	t.Parallel()
	tempDir := t.TempDir()

	tests := []struct {
		name            string
		initContent     string
		args            tools.Args
		expectedContent string
		errors          []error
	}{
		{
			name:            "nil",
			initContent:     "a\nb\nc",
			args:            nil,
			expectedContent: "a\nb\nc",
			errors:          []error{tools.ErrArguments},
		},
		{
			name:            "empty",
			initContent:     "a\nb\nc",
			args:            tools.Args{},
			expectedContent: "a\nb\nc",
			errors:          []error{tools.ErrArguments},
		},
		{
			name:        "all_args_insecure_path",
			initContent: "a\nb\nc",
			args: tools.Args{
				tools.ReplaceLinesV2Path:      "../../../../relative_path_test.txt",
				tools.ReplaceLinesV2StartLine: 1,
				tools.ReplaceLinesV2EndLine:   2,
				tools.ReplaceLinesV2Replace:   "1",
			},
			expectedContent: "a\nb\nc",
			errors:          []error{tools.ErrArguments, utils.ErrInsecureTargetPath},
		},
		{
			name:        "all_args_nonexistent_file",
			initContent: "a\nb\nc",
			args: tools.Args{
				tools.ReplaceLinesV2Path:      "garbage.txt",
				tools.ReplaceLinesV2StartLine: 1,
				tools.ReplaceLinesV2EndLine:   2,
				tools.ReplaceLinesV2Replace:   "test2",
			},
			expectedContent: "a\nb\nc",
			errors:          []error{tools.ErrFileSystem},
		},
		{
			name:        "path_replace_no_start_end",
			initContent: "a\nb\nc",
			args: tools.Args{
				tools.ReplaceLinesV2Path:    "path_replace_no_start_end_test.txt",
				tools.ReplaceLinesV2Replace: "a",
			},
			expectedContent: "a\nb\nc",
			errors:          []error{tools.ErrArguments},
		},
		{
			name:        "all_but_replace",
			initContent: "a\nb\nc",
			args: tools.Args{
				tools.ReplaceLinesV2Path:      "all_but_replace_test.txt",
				tools.ReplaceLinesV2StartLine: 1,
				tools.ReplaceLinesV2EndLine:   2,
			},
			expectedContent: "a\nb\nc",
			errors:          []error{tools.ErrArguments},
		},
		{
			name:        "end_before_start",
			initContent: "a\nb\nc",
			args: tools.Args{
				tools.ReplaceLinesV2Path:      "end_before_start_test.txt",
				tools.ReplaceLinesV2StartLine: 2,
				tools.ReplaceLinesV2EndLine:   1,
				tools.ReplaceLinesV2Replace:   "a",
			},
			expectedContent: "a\nb\nc",
			errors:          []error{tools.ErrArguments},
		},
		{
			name:        "zero_start",
			initContent: "a\nb\nc",
			args: tools.Args{
				tools.ReplaceLinesV2Path:      "zero_start_test.txt",
				tools.ReplaceLinesV2StartLine: 0,
				tools.ReplaceLinesV2EndLine:   1,
				tools.ReplaceLinesV2Replace:   "a",
			},
			expectedContent: "a\nb\nc",
			errors:          []error{tools.ErrArguments},
		},
		{
			name:        "zero_start_end",
			initContent: "a\nb\nc",
			args: tools.Args{
				tools.ReplaceLinesV2Path:      "zero_start_end_test.txt",
				tools.ReplaceLinesV2StartLine: 0,
				tools.ReplaceLinesV2EndLine:   0,
				tools.ReplaceLinesV2Replace:   "a",
			},
			expectedContent: "a\nb\nc",
			errors:          []error{tools.ErrArguments},
		},
		{
			name:        "end_beyond_file",
			initContent: "a\nb\nc",
			args: tools.Args{
				tools.ReplaceLinesV2Path:      "end_beyond_file_test.txt",
				tools.ReplaceLinesV2StartLine: 1,
				tools.ReplaceLinesV2EndLine:   7,
				tools.ReplaceLinesV2Replace:   "a",
			},
			expectedContent: "a\nb\nc",
			errors:          []error{tools.ErrArguments},
		},
		{
			name:        "one_line",
			initContent: "a\nb\nc",
			args: tools.Args{
				tools.ReplaceLinesV2Path:      "one_line_test.txt",
				tools.ReplaceLinesV2StartLine: 1,
				tools.ReplaceLinesV2EndLine:   1,
				tools.ReplaceLinesV2Replace:   "x\ny\n",
			},
			expectedContent: "x\ny\nb\nc",
			errors:          []error{},
		},
		{
			name:        "two_lines",
			initContent: "a\nb\nc\n",
			args: tools.Args{
				tools.ReplaceLinesV2Path:      "two_lines_test.txt",
				tools.ReplaceLinesV2StartLine: 2,
				tools.ReplaceLinesV2EndLine:   3,
				tools.ReplaceLinesV2Replace:   "x\ny\n",
			},
			expectedContent: "a\nx\ny\n",
			errors:          []error{},
		},
		{
			name:        "two_lines_with_trailing",
			initContent: "a\nb\nc\nd\n",
			args: tools.Args{
				tools.ReplaceLinesV2Path:      "two_lines_with_trailing_test.txt",
				tools.ReplaceLinesV2StartLine: 2,
				tools.ReplaceLinesV2EndLine:   3,
				tools.ReplaceLinesV2Replace:   "x\ny\n",
			},
			expectedContent: "a\nx\ny\nd\n",
			errors:          []error{},
		},
		{
			name:        "whole_text",
			initContent: "a\nb\nc\n",
			args: tools.Args{
				tools.ReplaceLinesV2Path:      "whole_text_test.txt",
				tools.ReplaceLinesV2StartLine: 1,
				tools.ReplaceLinesV2EndLine:   3,
				tools.ReplaceLinesV2Replace:   "x\ny\nz\n",
			},
			expectedContent: "x\ny\nz\n",
			errors:          []error{},
		},
		{
			name:        "delete_line",
			initContent: "a\nb\nc\n",
			args: tools.Args{
				tools.ReplaceLinesV2Path:      "delete_line_test.txt",
				tools.ReplaceLinesV2StartLine: 1,
				tools.ReplaceLinesV2EndLine:   1,
				tools.ReplaceLinesV2Replace:   "",
			},
			expectedContent: "b\nc\n",
			errors:          []error{},
		},
		{
			name:        "delete_lines",
			initContent: "a\nb\nc\n",
			args: tools.Args{
				tools.ReplaceLinesV2Path:      "delete_lines_test.txt",
				tools.ReplaceLinesV2StartLine: 1,
				tools.ReplaceLinesV2EndLine:   2,
				tools.ReplaceLinesV2Replace:   "",
			},
			expectedContent: "c\n",
			errors:          []error{},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			fileName := test.name + "_test.txt"
			tempPath := filepath.Join(tempDir, fileName)

			tempFile, tempFileErr := os.Create(tempPath)
			require.NoError(t, tempFileErr, "failed to create temporary file %q: %v", tempPath, tempFileErr)

			defer tempFile.Close()

			_, initContentErr := tempFile.WriteString(test.initContent)
			require.NoError(t, initContentErr, "failed to write initial content to file %q: %v", tempPath, initContentErr)

			rlt := &tools.ReplaceLinesV2Tool{ProjectPath: tempDir}

			_, runErr := rlt.Run(t.Context(), test.args)
			if test.errors == nil {
				require.NoError(t, runErr, "got unexpected run error")
			} else {
				for _, testErr := range test.errors {
					require.ErrorIs(t, runErr, testErr, "expected run error(s)")
				}
			}

			_, seekErr := tempFile.Seek(0, 0)
			require.NoError(t, seekErr, "failed to seek to start of temporary file %q: %v", tempPath, seekErr)

			result, readErr := io.ReadAll(tempFile)
			require.NoError(t, readErr, "failed to read temporary file %q: %v", tempPath, readErr)

			assert.Equal(t, []byte(test.expectedContent), result)
		})
	}
}
