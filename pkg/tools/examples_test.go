package tools_test

import (
	"testing"

	"github.com/cneill/smoke/pkg/tools"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExampleJSONParams(t *testing.T) { //nolint:funlen
	t.Parallel()

	tests := []struct {
		name     string
		args     []any
		expected string
		err      string
	}{
		{
			name: "invalid_arg_number",
			args: []any{"a"},
			err:  "example args must have key + val pairs, got non-even number of args",
		},
		{
			name: "invalid_arg_name",
			args: []any{1, 2},
			err:  "argument 0 was not a string (int): 1",
		},
		{
			name: "repeated_args",
			args: []any{"a", 1, "a", 2},
			err:  "got same argument (a) more than once",
		},
		{
			name:     "empty",
			args:     []any{},
			expected: "{}",
		},
		{
			name:     "int_arg",
			args:     []any{"a", 1},
			expected: `{"a":1}`,
		},
		{
			name:     "int_slice_arg",
			args:     []any{"a", []int{1, 2, 3}},
			expected: `{"a":[1,2,3]}`,
		},
		{
			name:     "string_slice_arg",
			args:     []any{"a", []string{"a", "b", "c"}},
			expected: `{"a":["a","b","c"]}`,
		},
		{
			name:     "two_args",
			args:     []any{"a", 1, "b", 2},
			expected: `{"a":1,"b":2}`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			output, err := tools.ExampleJSONParams(test.args...)
			if test.err == "" {
				require.NoError(t, err, "failed to generate JSON param example")
			} else {
				require.EqualError(t, err, test.err)
			}

			assert.Equal(t, test.expected, output, "mismatched JSON output vs. expected")
		})
	}
}
