package replacelines_test

import (
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/cneill/smoke/pkg/fs"
	"github.com/cneill/smoke/pkg/tools"
	"github.com/cneill/smoke/pkg/tools/handlers/replacelines"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReplaceLinesTool_Run(t *testing.T) { //nolint:funlen
	t.Parallel()
	tempDir := t.TempDir()
	rlt := &replacelines.ReplaceLines{ProjectPath: tempDir}

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
				replacelines.ParamPath:      "../../../../relative_path_test.txt",
				replacelines.ParamStartLine: 1,
				replacelines.ParamEndLine:   2,
				replacelines.ParamReplace:   "1",
			},
			expectedContent: "a\nb\nc",
			errors:          []error{tools.ErrArguments, fs.ErrInsecureTargetPath},
		},
		{
			name:        "all_args_nonexistent_file",
			initContent: "a\nb\nc",
			args: tools.Args{
				replacelines.ParamPath:      "garbage.txt",
				replacelines.ParamStartLine: 1,
				replacelines.ParamEndLine:   2,
				replacelines.ParamReplace:   "test2",
			},
			expectedContent: "a\nb\nc",
			errors:          []error{tools.ErrFileSystem},
		},
		{
			name:        "path_replace_no_start_end",
			initContent: "a\nb\nc",
			args: tools.Args{
				replacelines.ParamPath:    "path_replace_no_start_end_test.txt",
				replacelines.ParamReplace: "a",
			},
			expectedContent: "a\nb\nc",
			errors:          []error{tools.ErrArguments},
		},
		{
			name:        "all_but_replace",
			initContent: "a\nb\nc",
			args: tools.Args{
				replacelines.ParamPath:      "all_but_replace_test.txt",
				replacelines.ParamStartLine: 1,
				replacelines.ParamEndLine:   2,
			},
			expectedContent: "a\nb\nc",
			errors:          []error{tools.ErrArguments},
		},
		{
			name:        "end_before_start",
			initContent: "a\nb\nc",
			args: tools.Args{
				replacelines.ParamPath:      "end_before_start_test.txt",
				replacelines.ParamStartLine: 2,
				replacelines.ParamEndLine:   1,
				replacelines.ParamReplace:   "a",
			},
			expectedContent: "a\nb\nc",
			errors:          []error{tools.ErrArguments},
		},
		{
			name:        "negative_start",
			initContent: "a\nb\nc",
			args: tools.Args{
				replacelines.ParamPath:      "negative_start_test.txt",
				replacelines.ParamStartLine: -1,
				replacelines.ParamEndLine:   1,
				replacelines.ParamReplace:   "a",
			},
			expectedContent: "a\nb\nc",
			errors:          []error{tools.ErrArguments},
		},
		{
			name:        "negative_start_end",
			initContent: "a\nb\nc",
			args: tools.Args{
				replacelines.ParamPath:      "negative_start_end_test.txt",
				replacelines.ParamStartLine: -2,
				replacelines.ParamEndLine:   -1,
				replacelines.ParamReplace:   "a",
			},
			expectedContent: "a\nb\nc",
			errors:          []error{tools.ErrArguments},
		},
		{
			name:        "end_beyond_file",
			initContent: "a\nb\nc",
			args: tools.Args{
				replacelines.ParamPath:      "end_beyond_file_test.txt",
				replacelines.ParamStartLine: 1,
				replacelines.ParamEndLine:   7,
				replacelines.ParamReplace:   "a",
			},
			expectedContent: "a\nb\nc",
			errors:          []error{tools.ErrArguments},
		},
		{
			name:        "one_line",
			initContent: "a\nb\nc",
			args: tools.Args{
				replacelines.ParamPath:      "one_line_test.txt",
				replacelines.ParamStartLine: 1,
				replacelines.ParamEndLine:   1,
				replacelines.ParamReplace:   "x\ny\n",
			},
			expectedContent: "x\ny\nb\nc",
			errors:          []error{},
		},
		{
			name:        "two_lines",
			initContent: "a\nb\nc\n",
			args: tools.Args{
				replacelines.ParamPath:      "two_lines_test.txt",
				replacelines.ParamStartLine: 2,
				replacelines.ParamEndLine:   3,
				replacelines.ParamReplace:   "x\ny\n",
			},
			expectedContent: "a\nx\ny\n",
			errors:          []error{},
		},
		{
			name:        "two_lines_with_trailing",
			initContent: "a\nb\nc\nd\n",
			args: tools.Args{
				replacelines.ParamPath:      "two_lines_with_trailing_test.txt",
				replacelines.ParamStartLine: 2,
				replacelines.ParamEndLine:   3,
				replacelines.ParamReplace:   "x\ny\n",
			},
			expectedContent: "a\nx\ny\nd\n",
			errors:          []error{},
		},
		{
			name:        "whole_text",
			initContent: "a\nb\nc\n",
			args: tools.Args{
				replacelines.ParamPath:      "whole_text_test.txt",
				replacelines.ParamStartLine: 1,
				replacelines.ParamEndLine:   3,
				replacelines.ParamReplace:   "x\ny\nz\n",
			},
			expectedContent: "x\ny\nz\n",
			errors:          []error{},
		},
		{
			name:        "empty_file_init",
			initContent: "",
			args: tools.Args{
				replacelines.ParamPath:      "empty_file_init_test.txt",
				replacelines.ParamStartLine: 0,
				replacelines.ParamEndLine:   0,
				replacelines.ParamReplace:   "a\nb\nc\n",
			},
			expectedContent: "a\nb\nc\n",
			errors:          []error{},
		},
		{
			name:        "nonempty_file_init",
			initContent: "1\n2\n3\n",
			args: tools.Args{
				replacelines.ParamPath:      "nonempty_file_init_test.txt",
				replacelines.ParamStartLine: 0,
				replacelines.ParamEndLine:   0,
				replacelines.ParamReplace:   "a\nb\nc\n",
			},
			expectedContent: "a\nb\nc\n1\n2\n3\n",
			errors:          []error{},
		},

		{
			name:        "delete_line",
			initContent: "a\nb\nc\n",
			args: tools.Args{
				replacelines.ParamPath:      "delete_line_test.txt",
				replacelines.ParamStartLine: 1,
				replacelines.ParamEndLine:   1,
				replacelines.ParamReplace:   "",
			},
			expectedContent: "b\nc\n",
			errors:          []error{},
		},
		{
			name:        "delete_lines",
			initContent: "a\nb\nc\n",
			args: tools.Args{
				replacelines.ParamPath:      "delete_lines_test.txt",
				replacelines.ParamStartLine: 1,
				replacelines.ParamEndLine:   2,
				replacelines.ParamReplace:   "",
			},
			expectedContent: "c\n",
			errors:          []error{},
		},
		{
			name:        "delete_file",
			initContent: "a\nb\nc\n",
			args: tools.Args{
				replacelines.ParamPath:      "delete_file_test.txt",
				replacelines.ParamStartLine: 1,
				replacelines.ParamEndLine:   3,
				replacelines.ParamReplace:   "",
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

func TestReplaceLinesTool_ContextOutput(t *testing.T) { //nolint:funlen
	t.Parallel()

	tempDir := t.TempDir()
	rlt := &replacelines.ReplaceLines{ProjectPath: tempDir}
	initContent := "line1\nline2\nline3\nline4\n"
	emptyStr := ""

	tests := []struct {
		name             string
		initContent      *string
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
			name:        "empty_file_initialization",
			initContent: &emptyStr,
			startLine:   0,
			endLine:     0,
			replacement: "a\nb\nc\n",
			newLines:    [][]byte{},
			expectedContains: []string{
				"Added to top of file in \"empty_file_initialization_test.txt\"",
				"Context (lines 1-3):",
				"1: a",
				"2: b",
				"3: c",
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

			init := initContent
			if test.initContent != nil {
				init = *test.initContent
			}

			_, initContentErr := tempFile.WriteString(init)
			require.NoError(t, initContentErr, "failed to write initial content %q to file %q: %v", tempPath, initContent, initContentErr)

			args := tools.Args{
				replacelines.ParamPath:      fileName,
				replacelines.ParamStartLine: test.startLine,
				replacelines.ParamEndLine:   test.endLine,
				replacelines.ParamReplace:   test.replacement,
			}

			output, err := rlt.Run(t.Context(), args)
			require.NoError(t, err, "expected no error from Run()")

			for _, expected := range test.expectedContains {
				assert.Contains(t, output, expected, "expected output to contain %q", expected)
			}
		})
	}
}
