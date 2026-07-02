package prompts_test

import (
	"testing"

	"github.com/cneill/smoke/pkg/llmctx/prompts"

	"github.com/stretchr/testify/assert"
)

func TestMarkdownRenderer_Render(t *testing.T) {
	t.Parallel()

	builder := prompts.NewBuilder("test")

	builder.Add(
		prompts.SectionBackground,
		prompts.Hf(1, "Title %d", 1),
		prompts.Pf("This is a %s paragraph.", "leading"),
		prompts.Code("json", `{"test_attr": "value"}`),
		prompts.List(
			prompts.Item(
				"Item 1",
				prompts.Itemf("Item %d", 2),
			),
		),
	)

	preset := prompts.Preset{
		Name: "preset",
		Sections: map[prompts.SectionType]prompts.Nodes{
			prompts.SectionBackground: {
				prompts.H(1, "Title 2"),
				prompts.P("Addendum."),
			},
		},
	}

	builder.ApplyPreset(preset, prompts.Append)

	val := prompts.MarkdownRenderer{}.Render(builder.Build())
	assert.Contains(t, val, "## Background\n\n")
	assert.Contains(t, val, "### Title 1\n\n")
	assert.Contains(t, val, "```json")
	// make sure this gets added to the end
	assert.Contains(t, val, prompts.IndentStr+"* Item 2\n\n### Title 2\n\nAddendum.\n\n")
}
