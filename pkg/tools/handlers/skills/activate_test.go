package skills_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cneill/smoke/pkg/skills"
	"github.com/cneill/smoke/pkg/tools"
	skillshandler "github.com/cneill/smoke/pkg/tools/handlers/skills"
)

func testCatalog() skills.Catalog {
	return skills.Catalog{
		{
			Name:        "golang-testing",
			Description: "Best practices for writing Go tests",
			Body:        "Write table-driven tests.\nUse testify for assertions.\n",
		},
		{
			Name:        "code-review",
			Description: "Guidelines for code review",
			Body:        "Check for error handling.\nReview naming conventions.\n",
		},
	}
}

func newTool(catalog skills.Catalog) tools.Tool {
	tool, _ := skillshandler.New("", "")
	if catalog != nil {
		wsc, ok := tool.(tools.WantsSkillCatalog)
		if ok {
			wsc.SetSkillCatalog(catalog)
		}
	}

	return tool
}

func TestActivateSkill_Name(t *testing.T) {
	t.Parallel()

	tool := newTool(nil)
	assert.Equal(t, tools.NameActivateSkill, tool.Name())
}

func TestActivateSkill_DescriptionWithCatalog(t *testing.T) {
	t.Parallel()

	tool := newTool(testCatalog())
	desc := tool.Description()

	assert.Contains(t, desc, "golang-testing")
	assert.Contains(t, desc, "Best practices for writing Go tests")
	assert.Contains(t, desc, "code-review")
	assert.Contains(t, desc, "Guidelines for code review")
	assert.Contains(t, desc, "Available Skills")
}

func TestActivateSkill_DescriptionEmpty(t *testing.T) {
	t.Parallel()

	tool := newTool(nil)
	desc := tool.Description()

	assert.Contains(t, desc, "No skills are currently available")
	assert.Contains(t, desc, "## Examples")
}

func TestActivateSkill_Params(t *testing.T) {
	t.Parallel()

	tool := newTool(testCatalog())
	params := tool.Params()

	require.Len(t, params, 1)
	assert.Equal(t, "name", params[0].Key)
	assert.True(t, params[0].Required)
	assert.Equal(t, []string{"golang-testing", "code-review"}, params[0].EnumStringValues)
}

func TestActivateSkill_RunSuccess(t *testing.T) {
	t.Parallel()

	tool := newTool(testCatalog())
	args := tools.Args{"name": "golang-testing"}

	output, err := tool.Run(context.Background(), args)
	require.NoError(t, err)

	assert.Equal(t, "Write table-driven tests.\nUse testify for assertions.\n", output.Text)
}

func TestActivateSkill_RunUnknownSkill(t *testing.T) {
	t.Parallel()

	tool := newTool(testCatalog())
	args := tools.Args{"name": "nonexistent"}

	_, err := tool.Run(context.Background(), args)
	require.Error(t, err)
	assert.ErrorIs(t, err, tools.ErrArguments)
}

func TestActivateSkill_RunNoName(t *testing.T) {
	t.Parallel()

	tool := newTool(testCatalog())
	args := tools.Args{}

	_, err := tool.Run(context.Background(), args)
	require.Error(t, err)
	assert.ErrorIs(t, err, tools.ErrArguments)
}

func TestActivateSkill_RunEmptyBody(t *testing.T) {
	t.Parallel()

	catalog := skills.Catalog{
		{Name: "empty", Description: "An empty skill", Body: ""},
	}
	tool := newTool(catalog)
	args := tools.Args{"name": "empty"}

	output, err := tool.Run(context.Background(), args)
	require.NoError(t, err)
	assert.Contains(t, output.Text, "no body content")
}

func TestActivateSkill_SetSkillCatalog(t *testing.T) {
	t.Parallel()

	tool := newTool(nil)
	assert.Empty(t, tool.Params()[0].EnumStringValues)

	catalog := testCatalog()
	wsc, ok := tool.(tools.WantsSkillCatalog)
	require.True(t, ok)
	wsc.SetSkillCatalog(catalog)

	assert.Len(t, tool.Params()[0].EnumStringValues, 2)
}

func TestActivateSkill_Examples(t *testing.T) {
	t.Parallel()

	tool := newTool(testCatalog())
	examples := tool.Examples()

	require.Len(t, examples, 1)
	assert.Contains(t, examples[0].Description, "golang-testing")
}

func TestActivateSkill_ExamplesEmpty(t *testing.T) {
	t.Parallel()

	tool := newTool(nil)
	examples := tool.Examples()

	require.Len(t, examples, 1)
	assert.Contains(t, examples[0].Description, "my-skill")
}
