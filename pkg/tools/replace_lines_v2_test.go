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
	rlt := &tools.ReplaceLinesV2Tool{ProjectPath: tempDir}

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
		{
			name:        "delete_file",
			initContent: "a\nb\nc\n",
			args: tools.Args{
				tools.ReplaceLinesV2Path:      "delete_file_test.txt",
				tools.ReplaceLinesV2StartLine: 1,
				tools.ReplaceLinesV2EndLine:   3,
				tools.ReplaceLinesV2Replace:   "",
			},
			expectedContent: "",
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

func TestReplaceLinesV2Tool_ContextOutput(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	rlt := &tools.ReplaceLinesV2Tool{ProjectPath: tempDir}
	initContent := "line1\nline2\nline3\nline4\n"

	tests := []struct {
		name             string
		filePath         string
		startLine        int
		endLine          int
		replacement      string
		newLines         [][]byte
		expectedContains []string // Strings that should be in the output
		expectedSummary  string   // Expected summary line
	}{
		{
			name:        "single_line_replacement",
			startLine:   2,
			endLine:     2,
			replacement: "new line\n",
			newLines:    [][]byte{[]byte("line1"), []byte("new line"), []byte("line3"), []byte("line4")},
			expectedContains: []string{
				"Replaced line 2 in \"single_line_replacement_test.txt\".",
				"Context (lines 1-4):",
				"1: line1",
				"2: new line",
				"3: line3",
				"4: line4",
			},
		},
		{
			name:        "single_line_deletion",
			startLine:   2,
			endLine:     2,
			replacement: "",
			newLines:    [][]byte{[]byte("line1"), []byte("line3"), []byte("line4")},
			expectedContains: []string{
				"Deleted line 2 in \"single_line_deletion_test.txt\".",
				"Context (lines 1-3):",
				"1: line1",
				"2: line3",
				"3: line4",
			},
		},
		{
			name:        "multiple_line_replacement",
			startLine:   2,
			endLine:     3,
			replacement: "new line 1\nnew line 2\n",
			newLines:    [][]byte{[]byte("line1"), []byte("new line 1"), []byte("new line 2"), []byte("line4")},
			expectedContains: []string{
				"Replaced lines 2-3 in \"multiple_line_replacement_test.txt\".",
				"Context (lines 1-4):",
				"1: line1",
				"2: new line 1",
				"3: new line 2",
				"4: line4",
			},
		},
		{
			name:        "multiple_line_deletion",
			startLine:   2,
			endLine:     4,
			replacement: "",
			newLines:    [][]byte{[]byte("line1")},
			expectedContains: []string{
				"Deleted lines 2-4 in \"multiple_line_deletion_test.txt\".",
				"Context (line 1):",
				"1: line1",
			},
		},
		{
			name:        "empty_file_after_deletion",
			startLine:   1,
			endLine:     4,
			replacement: "",
			newLines:    [][]byte{},
			expectedContains: []string{
				"Deleted lines 1-4 in \"empty_file_after_deletion_test.txt\".",
				"(File is now empty)",
			},
		},
		{
			name:        "replacement_without_trailing_newline",
			startLine:   2,
			endLine:     2,
			replacement: "new line",
			newLines:    [][]byte{[]byte("line1"), []byte("new line"), []byte("line3")},
			expectedContains: []string{
				"Replaced line 2 in \"replacement_without_trailing_newline_test.txt\".",
				"Context (lines 1-4):",
				"1: line1",
				"2: new line",
				"3: line3",
				"4: line4",
			},
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

			_, initContentErr := tempFile.WriteString(initContent)
			require.NoError(t, initContentErr, "failed to write initial content to file %q: %v", tempPath, initContentErr)

			args := tools.Args{
				tools.ReplaceLinesV2Path:      fileName,
				tools.ReplaceLinesV2StartLine: test.startLine,
				tools.ReplaceLinesV2EndLine:   test.endLine,
				tools.ReplaceLinesV2Replace:   test.replacement,
			}

			output, err := rlt.Run(t.Context(), args)
			require.NoError(t, err, "expected no error from Run()")

			for _, expected := range test.expectedContains {
				assert.Contains(t, output, expected, "expected output to contain %q", expected)
			}
		})
	}
}
