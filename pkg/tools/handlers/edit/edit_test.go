package edit_test

import (
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/cneill/smoke/pkg/fs"
	"github.com/cneill/smoke/pkg/tools"
	"github.com/cneill/smoke/pkg/tools/handlers/edit"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEditToolRun(t *testing.T) { //nolint:funlen
	t.Parallel()

	tempDir := t.TempDir()
	tool := &edit.Edit{ProjectPath: tempDir}

	tests := []struct {
		name            string
		initContent     string
		args            tools.Args
		expectedContent string
		expectedOutput  string
		errorIs         []error
		errorContains   string
	}{
		{
			name:            "missing_path",
			initContent:     "alpha beta gamma",
			args:            tools.Args{},
			expectedContent: "alpha beta gamma",
			errorIs:         []error{tools.ErrArguments},
		},
		{
			name:        "insecure_path",
			initContent: "alpha beta gamma",
			args: tools.Args{
				edit.ParamPath:  "../outside.txt",
				edit.ParamEdits: []any{map[string]any{edit.ParamOldText: "alpha", edit.ParamNewText: "beta"}},
			},
			expectedContent: "alpha beta gamma",
			errorIs:         []error{tools.ErrArguments, fs.ErrInsecureTargetPath},
		},
		{
			name:        "missing_edits",
			initContent: "alpha beta gamma",
			args: tools.Args{
				edit.ParamPath: "missing_edits_test.txt",
			},
			expectedContent: "alpha beta gamma",
			errorIs:         []error{tools.ErrArguments},
		},
		{
			name:        "successful_multi_edit_merge",
			initContent: "alpha middle gamma tail",
			args: tools.Args{
				edit.ParamPath: "successful_multi_edit_merge_test.txt",
				edit.ParamEdits: []any{
					map[string]any{edit.ParamOldText: "alpha", edit.ParamNewText: "beta"},
					map[string]any{edit.ParamOldText: "gamma", edit.ParamNewText: "delta"},
				},
			},
			expectedContent: "beta middle delta tail",
			expectedOutput:  "Applied 2 edit(s) to \"successful_multi_edit_merge_test.txt\"",
		},
		{
			name:        "missing_match_errors",
			initContent: "alpha middle gamma tail",
			args: tools.Args{
				edit.ParamPath: "missing_match_errors_test.txt",
				edit.ParamEdits: []any{
					map[string]any{edit.ParamOldText: "omega", edit.ParamNewText: "delta"},
				},
			},
			expectedContent: "alpha middle gamma tail",
			errorIs:         []error{tools.ErrArguments},
			errorContains:   "not found",
		},
		{
			name:        "duplicate_match_errors",
			initContent: "alpha twice gamma",
			args: tools.Args{
				edit.ParamPath: "duplicate_match_errors_test.txt",
				edit.ParamEdits: []any{
					map[string]any{edit.ParamOldText: "a", edit.ParamNewText: "beta"},
				},
			},
			expectedContent: "alpha twice gamma",
			errorIs:         []error{tools.ErrArguments},
			errorContains:   "multiple times",
		},
		{
			name:        "overlapping_duplicate_match_errors",
			initContent: "ababa tail",
			args: tools.Args{
				edit.ParamPath: "overlapping_duplicate_match_errors_test.txt",
				edit.ParamEdits: []any{
					map[string]any{edit.ParamOldText: "aba", edit.ParamNewText: "xyz"},
				},
			},
			expectedContent: "ababa tail",
			errorIs:         []error{tools.ErrArguments},
			errorContains:   "multiple times",
		},
		{
			name:        "adjacent_edits_succeed",
			initContent: "abcdef",
			args: tools.Args{
				edit.ParamPath: "adjacent_edits_succeed_test.txt",
				edit.ParamEdits: []any{
					map[string]any{edit.ParamOldText: "abc", edit.ParamNewText: "123"},
					map[string]any{edit.ParamOldText: "def", edit.ParamNewText: "456"},
				},
			},
			expectedContent: "123456",
		},
		{
			name:        "empty_new_text_deletes_match",
			initContent: "alpha beta gamma",
			args: tools.Args{
				edit.ParamPath: "empty_new_text_deletes_match_test.txt",
				edit.ParamEdits: []any{
					map[string]any{edit.ParamOldText: " beta", edit.ParamNewText: ""},
				},
			},
			expectedContent: "alpha gamma",
		},
		{
			name:        "matches_are_based_on_original_contents",
			initContent: "alpha gamma",
			args: tools.Args{
				edit.ParamPath: "matches_are_based_on_original_contents_test.txt",
				edit.ParamEdits: []any{
					map[string]any{edit.ParamOldText: "alpha", edit.ParamNewText: "gamma"},
					map[string]any{edit.ParamOldText: "gamma", edit.ParamNewText: "delta"},
				},
			},
			expectedContent: "gamma delta",
		},
		{
			name:        "empty_old_text_errors",
			initContent: "alpha gamma",
			args: tools.Args{
				edit.ParamPath: "empty_old_text_errors_test.txt",
				edit.ParamEdits: []any{
					map[string]any{edit.ParamOldText: "", edit.ParamNewText: "delta"},
				},
			},
			expectedContent: "alpha gamma",
			errorIs:         []error{tools.ErrArguments},
			errorContains:   "must not be empty",
		},
		{
			name:        "duplicate_edits_same_span_error",
			initContent: "alpha gamma",
			args: tools.Args{
				edit.ParamPath: "duplicate_edits_same_span_error_test.txt",
				edit.ParamEdits: []any{
					map[string]any{edit.ParamOldText: "alpha", edit.ParamNewText: "beta"},
					map[string]any{edit.ParamOldText: "alpha", edit.ParamNewText: "omega"},
				},
			},
			expectedContent: "alpha gamma",
			errorIs:         []error{tools.ErrArguments},
			errorContains:   "overlaps",
		},
		{
			name:        "missing_new_text_errors",
			initContent: "alpha gamma",
			args: tools.Args{
				edit.ParamPath: "missing_new_text_errors_test.txt",
				edit.ParamEdits: []any{
					map[string]any{edit.ParamOldText: "alpha"},
				},
			},
			expectedContent: "alpha gamma",
			errorIs:         []error{tools.ErrArguments},
			errorContains:   "missing \"old_text\" or \"new_text\"",
		},
		{
			name:        "overlapping_edits_error",
			initContent: "abcdef",
			args: tools.Args{
				edit.ParamPath: "overlapping_edits_error_test.txt",
				edit.ParamEdits: []any{
					map[string]any{edit.ParamOldText: "abcd", edit.ParamNewText: "wxyz"},
					map[string]any{edit.ParamOldText: "cde", edit.ParamNewText: "123"},
				},
			},
			expectedContent: "abcdef",
			errorIs:         []error{tools.ErrArguments},
			errorContains:   "overlaps",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			fileName := test.name + "_test.txt"
			if path, ok := test.args[edit.ParamPath].(string); ok {
				fileName = path
			}

			tempPath := filepath.Join(tempDir, fileName)
			tempFile, err := os.Create(tempPath)
			require.NoError(t, err)

			defer tempFile.Close()

			_, err = tempFile.WriteString(test.initContent)
			require.NoError(t, err)

			output, runErr := tool.Run(t.Context(), test.args)
			if len(test.errorIs) == 0 {
				require.NoError(t, runErr)

				if test.expectedOutput != "" {
					require.NotNil(t, output)
					assert.Equal(t, test.expectedOutput, output.Text)
				}
			} else {
				for _, expectedErr := range test.errorIs {
					require.ErrorIs(t, runErr, expectedErr)
				}

				if test.errorContains != "" {
					require.ErrorContains(t, runErr, test.errorContains)
				}
			}

			_, err = tempFile.Seek(0, 0)
			require.NoError(t, err)

			result, err := io.ReadAll(tempFile)
			require.NoError(t, err)
			assert.Equal(t, test.expectedContent, string(result))
		})
	}
}
