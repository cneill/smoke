package llms_test

import (
	"encoding/json"
	"testing"

	"github.com/cneill/smoke/pkg/llms"
	"github.com/cneill/smoke/pkg/tools"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestToolCallArgsString(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		toolCall llms.ToolCall
		expected string
	}{
		{
			name: "raw_args_take_precedence",
			toolCall: llms.ToolCall{
				Args:    tools.Args{"path": "parsed"},
				RawArgs: `{"path":`,
			},
			expected: `{"path":`,
		},
		{
			name: "parsed_args",
			toolCall: llms.ToolCall{
				Args: tools.Args{"path": "README.md"},
			},
			expected: `{"path":"README.md"}`,
		},
		{
			name:     "nil_args",
			toolCall: llms.ToolCall{},
			expected: `{}`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, test.expected, test.toolCall.ArgsString())
		})
	}
}

func TestToolCallProviderArgs(t *testing.T) {
	t.Parallel()

	t.Run("valid_raw_json", func(t *testing.T) {
		t.Parallel()

		toolCall := llms.ToolCall{RawArgs: `{"path":"README.md"}`}
		providerArgs := toolCall.ProviderArgs()

		args, ok := providerArgs.(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "README.md", args["path"])
	})

	t.Run("invalid_raw_json", func(t *testing.T) {
		t.Parallel()

		toolCall := llms.ToolCall{RawArgs: `{"path":`}
		assert.Equal(t, `{"path":`, toolCall.ProviderArgs())
	})

	t.Run("parsed_args", func(t *testing.T) {
		t.Parallel()

		args := tools.Args{"path": "README.md"}
		toolCall := llms.ToolCall{Args: args}
		assert.Equal(t, args, toolCall.ProviderArgs())
	})
}

func TestToolCallInvalidArgs(t *testing.T) {
	t.Parallel()

	toolCall := llms.ToolCall{ArgsError: "invalid JSON"}

	assert.True(t, toolCall.InvalidArgs())
	require.Error(t, toolCall.GetArgsErr())
	assert.Equal(t, "invalid JSON", toolCall.GetArgsErr().Error())

	valid := llms.ToolCall{}
	assert.False(t, valid.InvalidArgs())
	assert.NoError(t, valid.GetArgsErr())
}

func TestToolCallJSONRoundTripIncludesRawArgsError(t *testing.T) {
	t.Parallel()

	original := llms.ToolCall{
		ID:        "call_123",
		Name:      "read_file",
		RawArgs:   `{"file_path":`,
		ArgsError: "invalid JSON",
	}

	data, err := json.Marshal(original)
	require.NoError(t, err)

	var restored llms.ToolCall
	require.NoError(t, json.Unmarshal(data, &restored))

	assert.Equal(t, original, restored)
}
