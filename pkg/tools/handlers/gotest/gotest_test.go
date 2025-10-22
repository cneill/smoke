package gotest_test

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/cneill/smoke/pkg/tools"
	"github.com/cneill/smoke/pkg/tools/handlers/gotest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testGoFile = "sum.go"

func writeModule(t *testing.T, root, pkg string, makeFail bool) {
	t.Helper()

	gomod := "module example.com/" + pkg + "\n\n" + "go 1.21\n"
	require.NoError(t, os.WriteFile(filepath.Join(root, "go.mod"), []byte(gomod), 0o644))

	src := "package " + pkg + "\n\nfunc Sum(a, b int) int { return a + b }\n"
	srcPath := filepath.Join(root, testGoFile)
	require.NoError(t, os.WriteFile(srcPath, []byte(src), 0o644))

	var want int
	if makeFail {
		want = 4
	} else {
		want = 3
	}

	testSrc := "package " + pkg + "\n\nimport \"testing\"\n\nfunc TestSum(t *testing.T) {\n\tif Sum(1, 2) != " +
		strconv.Itoa(want) + " {\n\t\tt.Fatalf(\"unexpected\")\n\t}\n}\n"
	require.NoError(t, os.WriteFile(filepath.Join(root, "sum_test.go"), []byte(testSrc), 0o644))
}

func TestGoTest_Run_NoPath_Passing(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	writeModule(t, tempDir, "p", false)

	tool := &gotest.GoTest{ProjectPath: tempDir}

	out, err := tool.Run(t.Context(), nil)
	require.NoError(t, err)
	assert.Contains(t, out.Text, "ok")
	assert.NotContains(t, out.Text, "FAIL")
}

func TestGoTest_Run_WithFilePath_Passing(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	writeModule(t, tempDir, "p2", false)

	tool := &gotest.GoTest{ProjectPath: tempDir}
	out, err := tool.Run(t.Context(), tools.Args{gotest.ParamPath: testGoFile})
	require.NoError(t, err)
	assert.Contains(t, out.Text, "ok")
	assert.NotContains(t, out.Text, "FAIL")
}

func TestGoTest_Run_FailingTest_ReturnsOutput(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	writeModule(t, tempDir, "p3", true)

	tool := &gotest.GoTest{ProjectPath: tempDir}
	out, err := tool.Run(t.Context(), nil)
	// Even with failing tests, error should be nil and output should include fail action.
	require.NoError(t, err)
	assert.NotEmpty(t, out.Text)
	assert.Contains(t, out.Text, "FAIL")
}

func TestGoTest_Run_InvalidAndMissingPaths(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	writeModule(t, tempDir, "p4", false)

	tool := &gotest.GoTest{ProjectPath: tempDir}

	// Non-existent in-project path
	out, err := tool.Run(t.Context(), tools.Args{gotest.ParamPath: "does_not_exist"})
	assert.Nil(t, out)
	// assert.Empty(t, out.Text)
	require.Error(t, err)
	require.ErrorIs(t, err, tools.ErrFileSystem)

	// Path outside project
	out, err = tool.Run(t.Context(), tools.Args{gotest.ParamPath: "../outside"})
	// assert.Empty(t, out.Text)
	assert.Nil(t, out)
	require.Error(t, err)
	require.ErrorIs(t, err, tools.ErrArguments)
}
