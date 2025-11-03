package tools_test

import (
	"testing"

	"github.com/cneill/smoke/pkg/tools"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExampleJSONParams(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		args     tools.Args
		expected string
		err      string
	}{
		{
			name:     "empty",
			args:     tools.Args{},
			expected: "{}",
		},
		{
			name:     "int_arg",
			args:     tools.Args{"a": 1},
			expected: `{"a":1}`,
		},
		{
			name:     "int_slice_arg",
			args:     tools.Args{"a": []int{1, 2, 3}},
			expected: `{"a":[1,2,3]}`,
		},
		{
			name:     "string_slice_arg",
			args:     tools.Args{"a": []string{"a", "b", "c"}},
			expected: `{"a":["a","b","c"]}`,
		},
		{
			name:     "two_args",
			args:     tools.Args{"a": 1, "b": 2},
			expected: `{"a":1,"b":2}`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			output, err := tools.ExampleJSONArguments(test.args)
			if test.err == "" {
				require.NoError(t, err, "failed to generate JSON param example")
			} else {
				require.EqualError(t, err, test.err)
			}

			assert.Equal(t, test.expected, output, "mismatched JSON output vs. expected")
		})
	}
}
