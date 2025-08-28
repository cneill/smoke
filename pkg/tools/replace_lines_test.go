package tools_test

//
// import (
// 	"errors"
// 	"io"
// 	"os"
// 	"path/filepath"
// 	"testing"
//
// 	"github.com/cneill/smoke/pkg/fs"
// 	"github.com/cneill/smoke/pkg/tools"
// )
//
// func TestReplaceLinesTool_Run(t *testing.T) { //nolint:cyclop,funlen
// 	t.Parallel()
//
// 	tempDir := t.TempDir()
//
// 	tests := []struct {
// 		name            string
// 		initContent     string
// 		args            tools.Args
// 		expectedContent string
// 		errors          []error
// 	}{
// 		{
// 			name:            "nil",
// 			initContent:     "a\nb\nc",
// 			args:            nil,
// 			expectedContent: "a\nb\nc",
// 			errors:          []error{tools.ErrArguments},
// 		},
// 		{
// 			name:            "empty",
// 			initContent:     "a\nb\nc",
// 			args:            tools.Args{},
// 			expectedContent: "a\nb\nc",
// 			errors:          []error{tools.ErrArguments},
// 		},
// 		{
// 			name:        "relative_path",
// 			initContent: "a\nb\nc",
// 			args: tools.Args{
// 				tools.ReplaceLinesPath:    "../../../../relative_path_test.txt",
// 				tools.ReplaceLinesSearch:  "a",
// 				tools.ReplaceLinesReplace: "1",
// 			},
// 			expectedContent: "a\nb\nc",
// 			errors:          []error{tools.ErrArguments, fs.ErrInsecureTargetPath},
// 		},
// 		{
// 			name:        "path_no_search_replace",
// 			initContent: "a\nb\nc",
// 			args: tools.Args{
// 				tools.ReplaceLinesPath: "path_no_search_replace_test.txt",
// 			},
// 			expectedContent: "a\nb\nc",
// 			errors:          []error{tools.ErrArguments},
// 		},
// 		{
// 			name:        "path_no_replace",
// 			initContent: "a\nb\nc",
// 			args: tools.Args{
// 				tools.ReplaceLinesPath:   "path_no_search_replace_test.txt",
// 				tools.ReplaceLinesSearch: "a",
// 			},
// 			expectedContent: "a\nb\nc",
// 			errors:          []error{tools.ErrArguments},
// 		},
// 		{
// 			name:        "path_no_search",
// 			initContent: "a\nb\nc",
// 			args: tools.Args{
// 				tools.ReplaceLinesPath:    "path_no_search_replace_test.txt",
// 				tools.ReplaceLinesReplace: "a",
// 			},
// 			expectedContent: "a\nb\nc",
// 			errors:          []error{tools.ErrArguments},
// 		},
// 		{
// 			name:        "mutually_exclusive",
// 			initContent: "a\nb\nc",
// 			args: tools.Args{
// 				tools.ReplaceLinesPath:    "mutually_exclusive_test.txt",
// 				tools.ReplaceLinesSearch:  "test",
// 				tools.ReplaceLinesReplace: "test2",
// 				tools.ReplaceLinesBatch:   []any{"test", "Test"},
// 			},
// 			expectedContent: "a\nb\nc",
// 			errors:          []error{tools.ErrArguments},
// 		},
// 		{
// 			name:        "empty_search_replace",
// 			initContent: "a\nb\nc",
// 			args: tools.Args{
// 				tools.ReplaceLinesPath:    "empty_search_replace_test.txt",
// 				tools.ReplaceLinesSearch:  "",
// 				tools.ReplaceLinesReplace: "",
// 			},
// 			expectedContent: "a\nb\nc",
// 			errors:          []error{tools.ErrArguments},
// 		},
// 		{
// 			name:        "empty_batch_search_replace",
// 			initContent: "a\nb\nc",
// 			args: tools.Args{
// 				tools.ReplaceLinesPath:  "mutually_exclusive_test.txt",
// 				tools.ReplaceLinesBatch: []string{"", "abc"},
// 			},
// 			expectedContent: "a\nb\nc",
// 			errors:          []error{tools.ErrArguments},
// 		},
// 		{
// 			name:        "int_batch",
// 			initContent: "a\nb\nc",
// 			args: tools.Args{
// 				tools.ReplaceLinesPath:  "mutually_exclusive_test.txt",
// 				tools.ReplaceLinesBatch: []int{1, 2, 3},
// 			},
// 			expectedContent: "a\nb\nc",
// 			errors:          []error{tools.ErrArguments},
// 		},
// 		{
// 			name:        "mismatched_types_any_batch",
// 			initContent: "a\nb\nc",
// 			args: tools.Args{
// 				tools.ReplaceLinesPath:  "mismatched_types_any_batch.txt",
// 				tools.ReplaceLinesBatch: []any{"a", "b", 1},
// 			},
// 			expectedContent: "a\nb\nc",
// 			errors:          []error{tools.ErrArguments},
// 		},
// 		{
// 			name:        "invalid_batch_size",
// 			initContent: "a\nb\nc",
// 			args: tools.Args{
// 				tools.ReplaceLinesPath:  "invalid_batch_size_test.txt",
// 				tools.ReplaceLinesBatch: []any{"a", "b", "c"},
// 			},
// 			expectedContent: "a\nb\nc",
// 			errors:          []error{tools.ErrArguments},
// 		},
// 		{
// 			name:        "all_args_bad_file",
// 			initContent: "a\nb\nc",
// 			args: tools.Args{
// 				tools.ReplaceLinesPath:    "garbage.txt",
// 				tools.ReplaceLinesSearch:  "test",
// 				tools.ReplaceLinesReplace: "test2",
// 			},
// 			expectedContent: "a\nb\nc",
// 			errors:          []error{tools.ErrFileSystem},
// 		},
// 		{
// 			name:        "no_replace",
// 			initContent: "a\nb\nc",
// 			args: tools.Args{
// 				tools.ReplaceLinesPath:    "no_replace_test.txt",
// 				tools.ReplaceLinesSearch:  "test",
// 				tools.ReplaceLinesReplace: "test2",
// 			},
// 			expectedContent: "a\nb\nc",
// 			errors:          nil,
// 		},
// 		{
// 			name:        "with_replace",
// 			initContent: "a\nb\nc",
// 			args: tools.Args{
// 				tools.ReplaceLinesPath:    "with_replace_test.txt",
// 				tools.ReplaceLinesSearch:  "a",
// 				tools.ReplaceLinesReplace: "1",
// 			},
// 			expectedContent: "1\nb\nc",
// 			errors:          nil,
// 		},
// 		{
// 			name:        "multiline_replace",
// 			initContent: "a\nb\nc",
// 			args: tools.Args{
// 				tools.ReplaceLinesPath:    "multiline_replace_test.txt",
// 				tools.ReplaceLinesSearch:  "a\nb",
// 				tools.ReplaceLinesReplace: "1\n2",
// 			},
// 			expectedContent: "1\n2\nc",
// 			errors:          nil,
// 		},
// 		{
// 			name:        "batch_string",
// 			initContent: "a\nb\nc",
// 			args: tools.Args{
// 				tools.ReplaceLinesPath:  "batch_string_test.txt",
// 				tools.ReplaceLinesBatch: []string{"a", "1", "b\nc", "2"},
// 			},
// 			expectedContent: "1\n2",
// 			errors:          nil,
// 		},
// 		{
// 			name:        "batch_any",
// 			initContent: "a\nb\nc",
// 			args: tools.Args{
// 				tools.ReplaceLinesPath:  "batch_any_test.txt",
// 				tools.ReplaceLinesBatch: []any{"a", "1", "b\nc", "2"},
// 			},
// 			expectedContent: "1\n2",
// 			errors:          nil,
// 		},
// 		{
// 			name:        "sequential_replace",
// 			initContent: "a\nb\nc",
// 			args: tools.Args{
// 				tools.ReplaceLinesPath:  "sequential_replace_test.txt",
// 				tools.ReplaceLinesBatch: []any{"a", "1", "1", "z"},
// 			},
// 			expectedContent: "z\nb\nc",
// 			errors:          nil,
// 		},
// 		{
// 			name:        "delete_lines",
// 			initContent: "a\nb\nc\n",
// 			args: tools.Args{
// 				tools.ReplaceLinesPath:    "delete_lines_test.txt",
// 				tools.ReplaceLinesSearch:  "a\nb\n",
// 				tools.ReplaceLinesReplace: "",
// 			},
// 			expectedContent: "c\n",
// 			errors:          nil,
// 		},
// 	}
//
// 	for _, test := range tests {
// 		t.Run(test.name, func(t *testing.T) {
// 			t.Parallel()
//
// 			fileName := test.name + "_test.txt"
// 			tempPath := filepath.Join(tempDir, fileName)
//
// 			tempFile, err := os.Create(tempPath)
// 			if err != nil {
// 				t.Fatalf("failed to create temporary file %q: %v", tempPath, err)
// 			}
//
// 			defer tempFile.Close()
//
// 			if _, err := tempFile.WriteString(test.initContent); err != nil {
// 				t.Fatalf("failed to write initial content to file %q: %v", tempPath, err)
// 			}
//
// 			rlt := &tools.ReplaceLinesTool{ProjectPath: tempDir}
//
// 			_, runErr := rlt.Run(t.Context(), test.args)
// 			if test.errors == nil && runErr != nil {
// 				t.Errorf("expected no error, got %v", runErr)
// 			} else if test.errors != nil {
// 				for _, testErr := range test.errors {
// 					if !errors.Is(runErr, testErr) {
// 						t.Errorf("expected error %v, got %v", testErr, runErr)
// 					}
// 				}
// 			}
//
// 			if _, err := tempFile.Seek(0, 0); err != nil {
// 				t.Errorf("failed to seek to start of temporary file %q: %v", tempPath, err)
// 			}
//
// 			result, err := io.ReadAll(tempFile)
// 			if err != nil {
// 				t.Errorf("failed to read temporary file %q: %v", tempPath, err)
// 			}
//
// 			if resultStr := string(result); resultStr != test.expectedContent {
// 				t.Errorf("returned contents %q don't match %q", resultStr, test.expectedContent)
// 			}
// 		})
// 	}
// }
