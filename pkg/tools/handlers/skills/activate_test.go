package skills_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cneill/smoke/pkg/llmctx/skills"
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

func TestActivateSkill_Run(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		catalog    skills.Catalog
		args       tools.Args
		wantOutput string
		wantErr    error
	}{
		{
			name:       "success",
			catalog:    testCatalog(),
			args:       tools.Args{"name": "golang-testing"},
			wantOutput: "Write table-driven tests.\nUse testify for assertions.\n",
		},
		{
			name:    "unknown_skill",
			catalog: testCatalog(),
			args:    tools.Args{"name": "nonexistent"},
			wantErr: tools.ErrArguments,
		},
		{
			name:    "no_name",
			catalog: testCatalog(),
			args:    tools.Args{},
			wantErr: tools.ErrArguments,
		},
		{
			name:       "empty_body",
			catalog:    skills.Catalog{{Name: "empty", Description: "An empty skill", Body: ""}},
			args:       tools.Args{"name": "empty"},
			wantOutput: "no body content",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			tool := newTool(test.catalog)

			output, err := tool.Run(context.Background(), test.args)
			if test.wantErr == nil {
				require.NoError(t, err)
				require.NotNil(t, output)
				assert.Contains(t, output.Text, test.wantOutput)
			} else {
				require.ErrorIs(t, err, test.wantErr)
			}
		})
	}
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

func TestActivateSkill_ExamplesEmpty(t *testing.T) {
	t.Parallel()

	tool := newTool(nil)
	examples := tool.Examples()

	require.Len(t, examples, 1)
	assert.Contains(t, examples[0].Description, "my-skill")
}
