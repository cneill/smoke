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
		params := tool.Params()
		examples := tool.Examples()

		t.Run(name+"_description", func(t *testing.T) {
			t.Parallel()

			description := tool.Description()
			assert.NotEmpty(t, description, "description for tool %s was empty", name)
			assert.Contains(t, description, "\n## Examples\n", "no examples in description for tool %s", name)
		})

		t.Run(name+"_params_description", func(t *testing.T) {
			t.Parallel()

			for _, param := range params {
				assert.NotEmpty(t, param.Description, "description for tool %s param %s was empty", name, param.Key)
			}
		})

		t.Run(name+"_examples", func(t *testing.T) {
			t.Parallel()

			requiredKeys := params.RequiredKeys()

			assert.NotEmpty(t, examples, "got zero examples for tool %s", name)

			for i, example := range examples {
				assert.NotEmpty(t, example.Description, "description for example %d from tool %s was empty", i, name)

				for _, key := range requiredKeys {
					assert.Contains(t, example.Args, key, "example %d missing required argument %s", i, key)
				}
			}
		})
	}
}
