package tools_test

import (
	"testing"

	"github.com/cneill/smoke/pkg/tools"

	"github.com/stretchr/testify/assert"
)

func TestAllToolDescriptions(t *testing.T) {
	t.Parallel()

	toolInits := tools.AllTools()
	testTools := make(tools.Tools, len(toolInits))

	for i, toolInit := range toolInits {
		testTools[i] = toolInit(".", "session")
	}

	for _, tool := range testTools {
		name := tool.Name()
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			assert.NotEmpty(t, tool.Description(), "description for tool "+name+" was empty")
		})

		params := tool.Params()
		for _, param := range params {
			assert.NotEmpty(t, param.Description, "description for tool "+name+" param "+param.Key+" was empty")
		}
	}
}
