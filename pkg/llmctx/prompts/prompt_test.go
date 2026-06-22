package prompts_test

import (
	"strings"
	"testing"

	"github.com/cneill/smoke/pkg/llmctx/prompts"

	"github.com/stretchr/testify/assert"
)

func TestListItems_RenderMarkdown(t *testing.T) {
	t.Parallel()

	list := prompts.List(
		prompts.Item("Item 1"),
		prompts.Item("Item 2"),
		prompts.Item("Item 3"),
	)

	var sb strings.Builder

	list.RenderMarkdown(&sb, 0)

	assert.Equal(t, "* Item 1\n* Item 2\n* Item 3\n\n", sb.String())

	nestedList := prompts.List(
		prompts.Item(
			"Parent 1",
			prompts.Item("Child 1"),
			prompts.Item(
				"Child 2",
				prompts.Itemf("Child %s", "2a"),
			),
		),
	)

	sb.Reset()

	nestedList.RenderMarkdown(&sb, 0)

	assert.Equal(t, "* Parent 1\n    * Child 1\n    * Child 2\n        * Child 2a\n\n", sb.String())
}
