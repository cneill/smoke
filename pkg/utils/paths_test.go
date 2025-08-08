package utils_test

import (
	"errors"
	"testing"

	"github.com/cneill/smoke/pkg/utils"
)

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
			targetPath:  "/a/b/c",
			expectedErr: utils.ErrInsecureProjectPath,
		},
		{
			name:        "root_project_path",
			projectPath: "/",
			targetPath:  "/a/b/c",
			expectedErr: utils.ErrInsecureProjectPath,
		},
		{
			name:        "relative_project_path",
			projectPath: "a/b/c",
			targetPath:  "/d/e/f",
			expectedErr: utils.ErrNonAbsoluteProjectPath,
		},
		{
			name:           "empty_target_path",
			projectPath:    "/a/b/c",
			targetPath:     "",
			expectedResult: "/a/b/c",
		},
		{
			name:           "dot_target_path",
			projectPath:    "/a/b/c",
			targetPath:     ".",
			expectedResult: "/a/b/c",
		},
		{
			name:        "double_dot_target_path",
			projectPath: "/a/b/c",
			targetPath:  "..",
			expectedErr: utils.ErrInsecureTargetPath,
		},
		{
			name:        "triple_dot_target_path",
			projectPath: "/a/b/c",
			targetPath:  "...",
			expectedErr: utils.ErrInsecureTargetPath,
		},
		{
			name:        "root_with_relative_lead",
			projectPath: "/a/b/c",
			targetPath:  "/..0",
			expectedErr: utils.ErrInsecureTargetPath,
		},
		{
			name:        "root_with_relative_lead",
			projectPath: "/a/b/c",
			targetPath:  "C:\\..0",
			expectedErr: utils.ErrInsecureTargetPath,
		},
		{
			name:        "relative_dir_access",
			projectPath: "/a/b/c",
			targetPath:  "../../../../etc/passwd",
			expectedErr: utils.ErrInsecureTargetPath,
		},
		{
			name:        "relative_windows_dir_access",
			projectPath: "/a/b/c",
			targetPath:  "..\\..\\..\\..\\etc\\passwd",
			expectedErr: utils.ErrInsecureTargetPath,
		},
		{
			name:        "extra_dots_relative_dir_access",
			projectPath: "/a/b/c",
			targetPath:  "./..././etc/passwd",
			expectedErr: utils.ErrInsecureTargetPath,
		},
		{
			name:           "extra_leading_dots",
			projectPath:    "/a/b/c",
			targetPath:     "./././d/e/f",
			expectedResult: "/a/b/c/d/e/f",
		},
		{
			name:           "extra_trailing_dots",
			projectPath:    "/a/b/c",
			targetPath:     "/d/e/f/./././",
			expectedResult: "/a/b/c/d/e/f",
		},
		{
			name:           "absolute_target_path",
			projectPath:    "/a/b/c",
			targetPath:     "/d/e/f",
			expectedResult: "/a/b/c/d/e/f",
		},
		{
			name:           "relative_target_path",
			projectPath:    "/a/b/c",
			targetPath:     "d/e/f/",
			expectedResult: "/a/b/c/d/e/f",
		},
		{
			name:           "target_path_double_separator_absolute",
			projectPath:    "/a/b/c",
			targetPath:     "//d//e//f",
			expectedResult: "/a/b/c/d/e/f",
		},
		{
			name:           "target_path_double_separator_relative",
			projectPath:    "/a/b/c",
			targetPath:     "d//e//f",
			expectedResult: "/a/b/c/d/e/f",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			result, err := utils.GetRelativePath(test.projectPath, test.targetPath)
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
	f.Add("/", "/a/b/c")
	f.Add("", "../../d/e/f")
	f.Add("a/b/c", "../../../../d/e/f/../../../")
	f.Add("C:\\project_dir", "/absolute/path")
	f.Add("C:\\absolute_path\\lol", "\\absolute\\path")
	f.Fuzz(func(t *testing.T, projectPath, targetPath string) {
		out, err := utils.GetRelativePath(projectPath, targetPath)
		if err != nil && errors.Is(err, utils.ErrOutsideBounds) {
			t.Errorf("%q, %v", out, err)
		}

		if err == nil && (out == "" || out == ".") {
			t.Errorf("%q", out)
		}
	})
}
