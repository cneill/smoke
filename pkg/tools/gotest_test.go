package tools_test

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/cneill/smoke/pkg/tools"
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

func TestGoTestTool_Run_NoPath_Passing(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	writeModule(t, tempDir, "p", false)

	tool := &tools.GoTestTool{ProjectPath: tempDir}

	out, err := tool.Run(nil)
	require.NoError(t, err)
	assert.Contains(t, out, "ok")
	assert.NotContains(t, out, "FAIL")
}

func TestGoTestTool_Run_WithFilePath_Passing(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	writeModule(t, tempDir, "p2", false)

	tool := &tools.GoTestTool{ProjectPath: tempDir}
	out, err := tool.Run(tools.Args{tools.GoTestPath: testGoFile})
	require.NoError(t, err)
	assert.Contains(t, out, "ok")
	assert.NotContains(t, out, "FAIL")
}

func TestGoTestTool_Run_FailingTest_ReturnsOutput(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	writeModule(t, tempDir, "p3", true)

	tool := &tools.GoTestTool{ProjectPath: tempDir}
	out, err := tool.Run(nil)
	// Even with failing tests, error should be nil and output should include fail action.
	require.NoError(t, err)
	assert.NotEmpty(t, out)
	assert.Contains(t, out, "FAIL")
}

func TestGoTestTool_Run_InvalidAndMissingPaths(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	writeModule(t, tempDir, "p4", false)

	tool := &tools.GoTestTool{ProjectPath: tempDir}

	// Non-existent in-project path
	out, err := tool.Run(tools.Args{tools.GoTestPath: "does_not_exist"})
	assert.Empty(t, out)
	require.Error(t, err)
	require.ErrorIs(t, err, tools.ErrFileSystem)

	// Path outside project
	out, err = tool.Run(tools.Args{tools.GoTestPath: "../outside"})
	assert.Empty(t, out)
	require.Error(t, err)
	require.ErrorIs(t, err, tools.ErrArguments)
}
