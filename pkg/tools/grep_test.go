package tools_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/cneill/smoke/pkg/tools"
)

func TestGrepTool_Run(t *testing.T) { //nolint:cyclop,funlen
	t.Parallel()

	tempDir := t.TempDir()

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
			if err != nil {
				t.Fatalf("failed to create temporary file %q: %v", tempPath, err)
			}

			defer tempFile.Close()

			if _, err := tempFile.WriteString(test.initContent); err != nil {
				t.Fatalf("failed to write initial content to file %q: %v", tempPath, err)
			}

			gt := &tools.GrepTool{ProjectPath: tempDir}

			output, runErr := gt.Run(test.args)
			if test.errors == nil && runErr != nil {
				t.Errorf("expected no error, got %v", runErr)
			} else if test.errors != nil {
				for _, testErr := range test.errors {
					if !errors.Is(runErr, testErr) {
						t.Errorf("expected error %v, got %v", testErr, runErr)
					}
				}
			}

			if output != test.expectedOutput {
				t.Errorf("returned output %q doesn't match expected %q", output, test.expectedOutput)
			}
		})
	}
}
