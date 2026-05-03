package grep_test

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/cneill/smoke/pkg/tools"
	"github.com/cneill/smoke/pkg/tools/formatting"
	"github.com/cneill/smoke/pkg/tools/handlers/grep"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGrep_Run(t *testing.T) { //nolint:funlen
	t.Parallel()

	tempDir := t.TempDir()
	grepTool := &grep.Grep{ProjectPath: tempDir}

	tests := []struct {
		name           string
		initContent    string
		args           tools.Args
		expectedOutput string
		errors         []error
	}{
		{
			name:           "nil",
			initContent:    "a\nb\nc",
			args:           nil,
			expectedOutput: "",
			errors:         []error{tools.ErrArguments},
		},
		{
			name:        "path_no_regex",
			initContent: "a\nb\nc",
			args: tools.Args{
				grep.ParamPath: "path_no_regex_test.txt",
			},
			expectedOutput: "",
			errors:         []error{tools.ErrArguments},
		},
		{
			name:        "regex_no_path",
			initContent: "a\nb\nc",
			args: tools.Args{
				grep.ParamRegex: `\w+`,
			},
			expectedOutput: "",
			errors:         []error{tools.ErrArguments},
		},
		{
			name:        "invalid_regex",
			initContent: "a\nb\nc",
			args: tools.Args{
				grep.ParamRegex: `(\w`,
				grep.ParamPath:  "invalid_regex_test.txt",
			},
			expectedOutput: "",
			errors:         []error{tools.ErrArguments},
		},
		{
			name:        "empty_regex",
			initContent: "a\nb\nc",
			args: tools.Args{
				grep.ParamRegex: "",
				grep.ParamPath:  "empty_regex_test.txt",
			},
			expectedOutput: "",
			errors:         []error{tools.ErrArguments},
		},
		{
			name:        "negative_context_lines",
			initContent: "a\nb\nc",
			args: tools.Args{
				grep.ParamRegex:        `\w`,
				grep.ParamPath:         "negative_context_lines.txt",
				grep.ParamContextLines: -1,
			},
			expectedOutput: "",
			errors:         []error{tools.ErrArguments},
		},
		// TODO: maybe check for an invalid value instead of just ignoring when non-int64?
		{
			name:        "invalid_context_lines",
			initContent: "a\nb\nc",
			args: tools.Args{
				grep.ParamRegex:        `\w`,
				grep.ParamPath:         "invalid_context_lines_test.txt",
				grep.ParamContextLines: "garbage",
			},
			expectedOutput: "invalid_context_lines_test.txt\n" + formatting.LineSep + "\n*1: a\n\n*2: b\n\n*3: c\n\n",
			errors:         nil,
		},
		{
			name:        "bad_path",
			initContent: "a\nb\nc",
			args: tools.Args{
				grep.ParamRegex: `\w+`,
				grep.ParamPath:  "random_path.txt",
			},
			expectedOutput: "",
			errors:         []error{tools.ErrFileSystem},
		},
		{
			name:        "no_match",
			initContent: "a\nb\nc",
			args: tools.Args{
				grep.ParamRegex: `xyz+`,
				grep.ParamPath:  "no_match_test.txt",
			},
			expectedOutput: "no_match_test.txt\n" + formatting.LineSep + "\n",
			errors:         nil,
		},
		{
			name:        "multiple_matches",
			initContent: "a\nb\nc",
			args: tools.Args{
				grep.ParamRegex: `[abc]`,
				grep.ParamPath:  "multiple_matches_test.txt",
			},
			expectedOutput: "multiple_matches_test.txt\n" + formatting.LineSep + "\n*1: a\n\n*2: b\n\n*3: c\n\n",
			errors:         nil,
		},
		{
			name:        "with_context_lines",
			initContent: "abc\n123\nxyz",
			args: tools.Args{
				grep.ParamRegex:        `123`,
				grep.ParamPath:         "with_context_lines_test.txt",
				grep.ParamContextLines: 2,
			},
			expectedOutput: "with_context_lines_test.txt\n" + formatting.LineSep + "\n1: abc\n*2: 123\n3: xyz\n\n",
			errors:         nil,
		},
		{
			name:        "binary_file",
			initContent: "\x00\n\x00\nmatch", //nolint:dupword
			args: tools.Args{
				grep.ParamRegex: `match`,
				grep.ParamPath:  "binary_file_test.txt",
			},
			expectedOutput: "binary_file_test.txt\n" + formatting.LineSep + "\n[binary file matches]\n\n",
			errors:         nil,
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

			output, runErr := grepTool.Run(t.Context(), test.args)
			if test.errors == nil {
				require.NoError(t, runErr, "unexpected error: %v", runErr)
			} else {
				for _, testErr := range test.errors {
					if !errors.Is(runErr, testErr) {
						require.ErrorIs(t, runErr, testErr)
					}
				}
			}

			if output != nil {
				assert.Equal(t, test.expectedOutput, output.Text)
			}
		})
	}
}

func TestGrep_Run_Directory(t *testing.T) { //nolint:funlen
	t.Parallel()

	tempDir := t.TempDir()
	grepTool := &grep.Grep{ProjectPath: tempDir}

	filePath1 := filepath.Join(tempDir, "file_1.txt")
	if err := os.WriteFile(filePath1, []byte("abc\n123\nxyz\n"), 0o644); err != nil {
		t.Fatalf("failed to create file %q: %v", filePath1, err)
	}

	filePath2 := filepath.Join(tempDir, "file_2.txt")
	if err := os.WriteFile(filePath2, []byte("test\ntest2\ntest3\n"), 0o644); err != nil {
		t.Fatalf("failed to create file %q: %v", filePath2, err)
	}

	filePath3 := filepath.Join(tempDir, "file_3.txt")
	if err := os.WriteFile(filePath3, []byte("123\n456\n789\n193\n"), 0o644); err != nil {
		t.Fatalf("failed to create file %q: %v", filePath3, err)
	}

	subdirPath := filepath.Join(tempDir, "subdir")
	if err := os.Mkdir(subdirPath, 0o755); err != nil {
		t.Fatalf("failed to create subdirectory %q: %v", subdirPath, err)
	}

	subdirFilePath := filepath.Join(tempDir, "subdir/file_4.txt")
	if err := os.WriteFile(subdirFilePath, []byte("123 123\ntest2\nxyz\nunique\n"), 0o644); err != nil {
		t.Fatalf("failed to create file %q: %v", subdirFilePath, err)
	}

	tests := []struct {
		name           string
		args           tools.Args
		expectedOutput string
		errors         []error
	}{
		{
			name: "no_match",
			args: tools.Args{
				grep.ParamPath:  ".",
				grep.ParamRegex: "NO_MATCH",
			},
			expectedOutput: "",
			errors:         nil,
		},
		{
			name: "single_match",
			args: tools.Args{
				grep.ParamPath:  "file_2.txt",
				grep.ParamRegex: "test2",
			},
			expectedOutput: fmt.Sprintf("file_2.txt\n%s\n*2: test2\n\n", formatting.LineSep),
			errors:         nil,
		},
		{
			name: "single_match_multi_file",
			args: tools.Args{
				grep.ParamPath:  ".",
				grep.ParamRegex: "123",
			},
			expectedOutput: fmt.Sprintf(
				"file_1.txt\n%s\n*2: 123\n\nfile_3.txt\n%s\n*1: 123\n\nsubdir/file_4.txt\n%s\n*1: 123 123\n\n",
				formatting.LineSep, formatting.LineSep, formatting.LineSep,
			),
			errors: nil,
		},
		{
			name: "single_match_subdir",
			args: tools.Args{
				grep.ParamPath:  "subdir",
				grep.ParamRegex: "test2",
			},
			expectedOutput: fmt.Sprintf("subdir/file_4.txt\n%s\n*2: test2\n\n", formatting.LineSep),
			errors:         nil,
		},
		{
			name: "multi_match_same_line_subdir",
			args: tools.Args{
				grep.ParamPath:  "subdir",
				grep.ParamRegex: "123",
			},
			expectedOutput: fmt.Sprintf("subdir/file_4.txt\n%s\n*1: 123 123\n\n", formatting.LineSep),
			errors:         nil,
		},
		{
			name: "multi_match_with_subdir",
			args: tools.Args{
				grep.ParamPath:  ".",
				grep.ParamRegex: `test(\d)?`,
			},
			expectedOutput: fmt.Sprintf(
				"file_2.txt\n%s\n*1: test\n\n*2: test2\n\n*3: test3\n\nsubdir/file_4.txt\n%s\n*2: test2\n\n",
				formatting.LineSep, formatting.LineSep,
			),
			errors: nil,
		},
		{
			name: "multi_match_multi_file",
			args: tools.Args{
				grep.ParamPath:  ".",
				grep.ParamRegex: `1(\d)3`,
			},
			expectedOutput: fmt.Sprintf(
				"file_1.txt\n%s\n*2: 123\n\nfile_3.txt\n%s\n*1: 123\n\n*4: 193\n\nsubdir/file_4.txt\n%s\n*1: 123 123\n\n",
				formatting.LineSep, formatting.LineSep, formatting.LineSep,
			),
			errors: nil,
		},
		{
			name: "no_multiline",
			args: tools.Args{
				grep.ParamPath:  ".",
				grep.ParamRegex: "test\ntest2",
			},
			expectedOutput: "",
			errors:         nil,
		},
		{
			name: "multi_match_multi_file_with_context",
			args: tools.Args{
				grep.ParamPath:         ".",
				grep.ParamRegex:        `1(\d)3`,
				grep.ParamContextLines: 2,
			},
			expectedOutput: fmt.Sprintf(
				"file_1.txt\n%s\n1: abc\n*2: 123\n3: xyz\n\nfile_3.txt\n%s\n*1: 123\n2: 456\n3: 789\n\n2: 456\n3: 789\n*4: 193\n\n"+
					"subdir/file_4.txt\n%s\n*1: 123 123\n2: test2\n3: xyz\n\n",
				formatting.LineSep, formatting.LineSep, formatting.LineSep,
			),
			errors: nil,
		},
		{
			name: "multi_match_multi_file_with_long_context",
			args: tools.Args{
				grep.ParamPath:         ".",
				grep.ParamRegex:        `1(\d)3`,
				grep.ParamContextLines: 10,
			},
			expectedOutput: fmt.Sprintf(
				"file_1.txt\n%s\n1: abc\n*2: 123\n3: xyz\n\n"+
					"file_3.txt\n%s\n*1: 123\n2: 456\n3: 789\n4: 193\n\n1: 123\n2: 456\n3: 789\n*4: 193\n\n"+
					"subdir/file_4.txt\n%s\n*1: 123 123\n2: test2\n3: xyz\n4: unique\n\n",
				formatting.LineSep, formatting.LineSep, formatting.LineSep,
			),
			errors: nil,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			output, runErr := grepTool.Run(t.Context(), test.args)
			if test.errors == nil {
				require.NoError(t, runErr)
			} else {
				for _, testErr := range test.errors {
					require.ErrorIs(t, runErr, testErr)
				}
			}

			if output != nil {
				assert.Equal(t, test.expectedOutput, output.Text, "returned output doesn't match expected")
			}
		})
	}
}
