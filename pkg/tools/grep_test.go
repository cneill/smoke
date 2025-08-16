package tools_test

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/cneill/smoke/pkg/tools"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGrepTool_Run(t *testing.T) { //nolint:funlen
	t.Parallel()

	tempDir := t.TempDir()
	grepTool := &tools.GrepTool{ProjectPath: tempDir}

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
				tools.GrepPath: "path_no_regex_test.txt",
			},
			expectedOutput: "",
			errors:         []error{tools.ErrArguments},
		},
		{
			name:        "regex_no_path",
			initContent: "a\nb\nc",
			args: tools.Args{
				tools.GrepRegex: `\w+`,
			},
			expectedOutput: "",
			errors:         []error{tools.ErrArguments},
		},
		{
			name:        "invalid_regex",
			initContent: "a\nb\nc",
			args: tools.Args{
				tools.GrepRegex: `(\w`,
				tools.GrepPath:  "invalid_regex_test.txt",
			},
			expectedOutput: "",
			errors:         []error{tools.ErrArguments},
		},
		{
			name:        "empty_regex",
			initContent: "a\nb\nc",
			args: tools.Args{
				tools.GrepRegex: "",
				tools.GrepPath:  "empty_regex_test.txt",
			},
			expectedOutput: "",
			errors:         []error{tools.ErrArguments},
		},
		{
			name:        "negative_context_lines",
			initContent: "a\nb\nc",
			args: tools.Args{
				tools.GrepRegex:        `\w`,
				tools.GrepPath:         "negative_context_lines.txt",
				tools.GrepContextLines: -1,
			},
			expectedOutput: "",
			errors:         []error{tools.ErrArguments},
		},
		// TODO: maybe check for an invalid value instead of just ignoring when non-int64?
		{
			name:        "invalid_context_lines",
			initContent: "a\nb\nc",
			args: tools.Args{
				tools.GrepRegex:        `\w`,
				tools.GrepPath:         "invalid_context_lines_test.txt",
				tools.GrepContextLines: "garbage",
			},
			expectedOutput: "invalid_context_lines_test.txt\n" + tools.LineSep + "\n*1: a\n\n*2: b\n\n*3: c\n\n",
			errors:         nil,
		},
		{
			name:        "bad_path",
			initContent: "a\nb\nc",
			args: tools.Args{
				tools.GrepRegex: `\w+`,
				tools.GrepPath:  "random_path.txt",
			},
			expectedOutput: "",
			errors:         []error{tools.ErrFileSystem},
		},
		{
			name:        "no_match",
			initContent: "a\nb\nc",
			args: tools.Args{
				tools.GrepRegex: `xyz+`,
				tools.GrepPath:  "no_match_test.txt",
			},
			expectedOutput: "no_match_test.txt\n" + tools.LineSep + "\n",
			errors:         nil,
		},
		{
			name:        "multiple_matches",
			initContent: "a\na\na",
			args: tools.Args{
				tools.GrepRegex: `a`,
				tools.GrepPath:  "multiple_matches_test.txt",
			},
			expectedOutput: "multiple_matches_test.txt\n" + tools.LineSep + "\n*1: a\n\n*2: a\n\n*3: a\n\n",
			errors:         nil,
		},
		{
			name:        "with_context_lines",
			initContent: "abc\n123\nxyz",
			args: tools.Args{
				tools.GrepRegex:        `123`,
				tools.GrepPath:         "with_context_lines_test.txt",
				tools.GrepContextLines: 2,
			},
			expectedOutput: "with_context_lines_test.txt\n" + tools.LineSep + "\n1: abc\n*2: 123\n3: xyz\n\n",
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

			output, runErr := grepTool.Run(test.args)
			if test.errors == nil {
				require.NoError(t, runErr, "unexpected error: %v", runErr)
			} else {
				for _, testErr := range test.errors {
					if !errors.Is(runErr, testErr) {
						require.ErrorIs(t, runErr, testErr)
					}
				}
			}

			assert.Equal(t, test.expectedOutput, output)
		})
	}
}

func TestGrepTool_Run_Directory(t *testing.T) { //nolint:funlen
	t.Parallel()

	tempDir := t.TempDir()
	grepTool := &tools.GrepTool{ProjectPath: tempDir}

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
				tools.GrepPath:  ".",
				tools.GrepRegex: "NO_MATCH",
			},
			expectedOutput: "",
			errors:         nil,
		},
		{
			name: "single_match",
			args: tools.Args{
				tools.GrepPath:  "file_2.txt",
				tools.GrepRegex: "test2",
			},
			expectedOutput: fmt.Sprintf("file_2.txt\n%s\n*2: test2\n\n", tools.LineSep),
			errors:         nil,
		},
		{
			name: "single_match_multi_file",
			args: tools.Args{
				tools.GrepPath:  ".",
				tools.GrepRegex: "123",
			},
			expectedOutput: fmt.Sprintf(
				"file_1.txt\n%s\n*2: 123\n\nfile_3.txt\n%s\n*1: 123\n\nsubdir/file_4.txt\n%s\n*1: 123 123\n\n",
				tools.LineSep, tools.LineSep, tools.LineSep,
			),
			errors: nil,
		},
		{
			name: "single_match_subdir",
			args: tools.Args{
				tools.GrepPath:  "subdir",
				tools.GrepRegex: "test2",
			},
			expectedOutput: fmt.Sprintf("subdir/file_4.txt\n%s\n*2: test2\n\n", tools.LineSep),
			errors:         nil,
		},
		{
			name: "multi_match_same_line_subdir",
			args: tools.Args{
				tools.GrepPath:  "subdir",
				tools.GrepRegex: "123",
			},
			expectedOutput: fmt.Sprintf("subdir/file_4.txt\n%s\n*1: 123 123\n\n", tools.LineSep),
			errors:         nil,
		},
		{
			name: "multi_match_with_subdir",
			args: tools.Args{
				tools.GrepPath:  ".",
				tools.GrepRegex: `test(\d)?`,
			},
			expectedOutput: fmt.Sprintf(
				"file_2.txt\n%s\n*1: test\n\n*2: test2\n\n*3: test3\n\nsubdir/file_4.txt\n%s\n*2: test2\n\n",
				tools.LineSep, tools.LineSep,
			),
			errors: nil,
		},
		{
			name: "multi_match_multi_file",
			args: tools.Args{
				tools.GrepPath:  ".",
				tools.GrepRegex: `1(\d)3`,
			},
			expectedOutput: fmt.Sprintf(
				"file_1.txt\n%s\n*2: 123\n\nfile_3.txt\n%s\n*1: 123\n\n*4: 193\n\nsubdir/file_4.txt\n%s\n*1: 123 123\n\n",
				tools.LineSep, tools.LineSep, tools.LineSep,
			),
			errors: nil,
		},
		{
			name: "no_multiline",
			args: tools.Args{
				tools.GrepPath:  ".",
				tools.GrepRegex: "test\ntest2",
			},
			expectedOutput: "",
			errors:         nil,
		},
		{
			name: "multi_match_multi_file_with_context",
			args: tools.Args{
				tools.GrepPath:         ".",
				tools.GrepRegex:        `1(\d)3`,
				tools.GrepContextLines: 2,
			},
			expectedOutput: fmt.Sprintf(
				"file_1.txt\n%s\n1: abc\n*2: 123\n3: xyz\n\nfile_3.txt\n%s\n*1: 123\n2: 456\n3: 789\n\n2: 456\n3: 789\n*4: 193\n\n"+
					"subdir/file_4.txt\n%s\n*1: 123 123\n2: test2\n3: xyz\n\n",
				tools.LineSep, tools.LineSep, tools.LineSep,
			),
			errors: nil,
		},
		{
			name: "multi_match_multi_file_with_long_context",
			args: tools.Args{
				tools.GrepPath:         ".",
				tools.GrepRegex:        `1(\d)3`,
				tools.GrepContextLines: 10,
			},
			expectedOutput: fmt.Sprintf(
				"file_1.txt\n%s\n1: abc\n*2: 123\n3: xyz\n\n"+
					"file_3.txt\n%s\n*1: 123\n2: 456\n3: 789\n4: 193\n\n1: 123\n2: 456\n3: 789\n*4: 193\n\n"+
					"subdir/file_4.txt\n%s\n*1: 123 123\n2: test2\n3: xyz\n4: unique\n\n",
				tools.LineSep, tools.LineSep, tools.LineSep,
			),
			errors: nil,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			output, runErr := grepTool.Run(test.args)
			if test.errors == nil {
				require.NoError(t, runErr)
			} else {
				for _, testErr := range test.errors {
					require.ErrorIs(t, runErr, testErr)
				}
			}

			assert.Equal(t, test.expectedOutput, output, "returned output doesn't match expected")
		})
	}
}
