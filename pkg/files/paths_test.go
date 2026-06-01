package files_test

import (
	"errors"
	"testing"

	"github.com/cneill/smoke/pkg/files"
)

const testRoot = "/a/b/c"

func TestGetRelative(t *testing.T) { //nolint:funlen
	t.Parallel()

	tests := []struct {
		name           string
		projectPath    string
		targetPath     string
		expectedResult string
		expectedErr    error
	}{
		{
			name:        "empty_project_path",
			projectPath: "",
			targetPath:  testRoot,
			expectedErr: files.ErrInsecureProjectPath,
		},
		{
			name:        "root_project_path",
			projectPath: "/",
			targetPath:  testRoot,
			expectedErr: files.ErrInsecureProjectPath,
		},
		{
			name:        "relative_project_path",
			projectPath: "a/b/c",
			targetPath:  "/d/e/f",
			expectedErr: files.ErrNonAbsoluteProjectPath,
		},
		{
			name:           "empty_target_path",
			projectPath:    testRoot,
			targetPath:     "",
			expectedResult: testRoot,
		},
		{
			name:           "dot_target_path",
			projectPath:    testRoot,
			targetPath:     ".",
			expectedResult: testRoot,
		},
		{
			name:        "double_dot_target_path",
			projectPath: testRoot,
			targetPath:  "..",
			expectedErr: files.ErrInsecureTargetPath,
		},
		{
			name:        "triple_dot_target_path",
			projectPath: testRoot,
			targetPath:  "...",
			expectedErr: files.ErrInsecureTargetPath,
		},
		{
			name:        "root_with_relative_lead",
			projectPath: testRoot,
			targetPath:  "/..0",
			expectedErr: files.ErrInsecureTargetPath,
		},
		{
			name:        "root_with_relative_lead",
			projectPath: testRoot,
			targetPath:  "C:\\..0",
			expectedErr: files.ErrInsecureTargetPath,
		},
		{
			name:        "relative_dir_access",
			projectPath: testRoot,
			targetPath:  "../../../../etc/passwd",
			expectedErr: files.ErrInsecureTargetPath,
		},
		{
			name:        "relative_windows_dir_access",
			projectPath: testRoot,
			targetPath:  "..\\..\\..\\..\\etc\\passwd",
			expectedErr: files.ErrInsecureTargetPath,
		},
		{
			name:        "extra_dots_relative_dir_access",
			projectPath: testRoot,
			targetPath:  "./..././etc/passwd",
			expectedErr: files.ErrInsecureTargetPath,
		},
		{
			name:           "extra_leading_dots",
			projectPath:    testRoot,
			targetPath:     "./././d/e/f",
			expectedResult: "/a/b/c/d/e/f",
		},
		{
			name:           "extra_trailing_dots",
			projectPath:    testRoot,
			targetPath:     "/d/e/f/./././",
			expectedResult: "/a/b/c/d/e/f",
		},
		{
			name:           "absolute_target_path",
			projectPath:    testRoot,
			targetPath:     "/d/e/f",
			expectedResult: "/a/b/c/d/e/f",
		},
		{
			name:           "relative_target_path",
			projectPath:    testRoot,
			targetPath:     "d/e/f/",
			expectedResult: "/a/b/c/d/e/f",
		},
		{
			name:           "target_path_double_separator_absolute",
			projectPath:    testRoot,
			targetPath:     "//d//e//f",
			expectedResult: "/a/b/c/d/e/f",
		},
		{
			name:           "target_path_double_separator_relative",
			projectPath:    testRoot,
			targetPath:     "d//e//f",
			expectedResult: "/a/b/c/d/e/f",
		},
		// {
		// 	name:           "target_path_null_byte",
		// 	projectPath:    testRoot,
		// 	targetPath:     "d/\x00/f",
		// 	expectedResult: "/a/b/c/d/f",
		// },
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			result, err := files.GetRelativePath(test.projectPath, test.targetPath)
			if test.expectedErr != nil {
				if !errors.Is(err, test.expectedErr) {
					t.Fatalf("expecting error %v, got %v", test.expectedErr, err)
				}
			} else if test.expectedResult != result {
				t.Fatalf("expecting result %q, got %q", test.expectedResult, result)
			}
		})
	}
}

func FuzzGetRelative(f *testing.F) {
	f.Add("/", testRoot)
	f.Add("", "../../d/e/f")
	f.Add("a/b/c", "../../../../d/e/f/../../../")
	f.Add("C:\\project_dir", "/absolute/path")
	f.Add("C:\\absolute_path\\lol", "\\absolute\\path")
	f.Fuzz(func(t *testing.T, projectPath, targetPath string) {
		out, err := files.GetRelativePath(projectPath, targetPath)
		if err != nil && errors.Is(err, files.ErrOutsideBounds) {
			t.Errorf("%q, %v", out, err)
		}

		if err == nil && (out == "" || out == ".") {
			t.Errorf("%q", out)
		}
	})
}
