package tools_test

import (
	"testing"

	"github.com/cneill/smoke/pkg/tools"
)

func TestArgs_GetStringSlice(t *testing.T) { //nolint:funlen
	t.Parallel()

	testKey := "test"
	rawArgs := `{"test": ["1", "2", "3"]}`

	parsedArgs, err := tools.GetArgs([]byte(rawArgs), tools.Params{
		{
			Key:      testKey,
			Type:     tools.ParamTypeArray,
			ItemType: tools.ParamTypeString,
		},
	})
	if err != nil {
		t.Fatalf("failed to get args for test case: %v", err)
	}

	tests := []struct {
		name     string
		args     tools.Args
		expected []string
	}{
		{
			name:     "empty",
			args:     tools.Args{},
			expected: nil,
		},
		{
			name:     "strings",
			args:     tools.Args{testKey: []any{"1", "2", "3"}},
			expected: []string{"1", "2", "3"},
		},
		{
			name:     "strings",
			args:     tools.Args{testKey: []string{"1", "2", "3"}},
			expected: []string{"1", "2", "3"},
		},
		{
			name:     "parsed_args",
			args:     parsedArgs,
			expected: []string{"1", "2", "3"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			result := test.args.GetStringSlice(testKey)
			if len(result) != len(test.expected) {
				t.Errorf("mismatched lengths: got %d, expected %d", len(result), len(test.expected))
			} else if (result == nil) != (test.expected == nil) {
				t.Errorf("got %v, expected %v", result, test.expected)
			}

			if result == nil || test.expected == nil {
				return
			}

			for i := range test.expected {
				if result[i] != test.expected[i] {
					t.Errorf("mismatch in position %d: got %s, expected %s", i, test.expected[i], result[i])
				}
			}
		})
	}
}
