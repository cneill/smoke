package tools_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/cneill/smoke/pkg/tools"
	"github.com/stretchr/testify/assert"
)

const testKey = "test"

func TestArgs_GetString(t *testing.T) {
	t.Parallel()

	strPtr := func(input string) *string {
		return &input
	}

	tests := []struct {
		name     string
		args     tools.Args
		expected *string
	}{
		{
			name:     "nil",
			args:     nil,
			expected: nil,
		},
		{
			name:     "empty",
			args:     tools.Args{},
			expected: nil,
		},
		{
			name:     "bool",
			args:     tools.Args{testKey: true},
			expected: nil,
		},
		{
			name:     "float",
			args:     tools.Args{testKey: 1.2},
			expected: nil,
		},
		{
			name:     "string",
			args:     tools.Args{testKey: "test"},
			expected: strPtr("test"),
		},
		{
			name:     "stringer",
			args:     tools.Args{testKey: time.Date(2025, time.January, 1, 0, 0, 0, 0, time.UTC)},
			expected: strPtr("2025-01-01 00:00:00 +0000 UTC"),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			result := test.args.GetString(testKey)

			assert.Equal(t, test.expected == nil, result == nil)

			if test.expected != nil && result != nil {
				assert.Equal(t, *test.expected, *result)
			}
		})
	}
}

func TestArgs_GetInt_and_GetInt64(t *testing.T) { //nolint:funlen
	t.Parallel()

	intPtr := func(input int) *int {
		return &input
	}

	int64Ptr := func(input int64) *int64 {
		return &input
	}

	tests := []struct {
		name          string
		args          tools.Args
		expectedInt   *int
		expectedInt64 *int64
	}{
		{
			name:          "nil",
			args:          nil,
			expectedInt:   nil,
			expectedInt64: nil,
		},
		{
			name:          "empty",
			args:          tools.Args{},
			expectedInt:   nil,
			expectedInt64: nil,
		},
		{
			name:          "bool",
			args:          tools.Args{testKey: true},
			expectedInt:   nil,
			expectedInt64: nil,
		},
		{
			name:          "float_str",
			args:          tools.Args{testKey: "1.2"},
			expectedInt:   nil,
			expectedInt64: nil,
		},
		{
			name:          "garbage_str",
			args:          tools.Args{testKey: "garbage"},
			expectedInt:   nil,
			expectedInt64: nil,
		},
		{
			name:          "json_number_float",
			args:          tools.Args{testKey: json.Number("1.5")},
			expectedInt:   nil,
			expectedInt64: nil,
		},
		{
			name:          "int",
			args:          tools.Args{testKey: int(1)},
			expectedInt:   intPtr(1),
			expectedInt64: int64Ptr(1),
		},
		{
			name:          "int64",
			args:          tools.Args{testKey: int64(1)},
			expectedInt:   intPtr(1),
			expectedInt64: int64Ptr(1),
		},
		{
			name:          "string",
			args:          tools.Args{testKey: "1"},
			expectedInt:   intPtr(1),
			expectedInt64: int64Ptr(1),
		},
		{
			name:          "json_number_int",
			args:          tools.Args{testKey: json.Number("1")},
			expectedInt:   intPtr(1),
			expectedInt64: int64Ptr(1),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			intResult := test.args.GetInt(testKey)
			int64Result := test.args.GetInt64(testKey)

			assert.Equal(t, test.expectedInt, intResult, "unexpected int value")
			assert.Equal(t, test.expectedInt64, int64Result, "unexpected int64 value")
		})
	}
}

func TestArgs_GetFloat64(t *testing.T) { //nolint:funlen
	t.Parallel()

	float64Ptr := func(input float64) *float64 {
		return &input
	}

	tests := []struct {
		name     string
		args     tools.Args
		expected *float64
	}{
		{
			name:     "nil",
			args:     nil,
			expected: nil,
		},
		{
			name:     "empty",
			args:     tools.Args{},
			expected: nil,
		},
		{
			name:     "bool",
			args:     tools.Args{testKey: true},
			expected: nil,
		},
		{
			name:     "garbage_str",
			args:     tools.Args{testKey: "garbage"},
			expected: nil,
		},
		{
			name:     "int",
			args:     tools.Args{testKey: int(1)},
			expected: float64Ptr(1),
		},
		{
			name:     "int64",
			args:     tools.Args{testKey: int64(1)},
			expected: float64Ptr(1),
		},
		{
			name:     "int_str",
			args:     tools.Args{testKey: "1"},
			expected: float64Ptr(1.0),
		},
		{
			name:     "float_str",
			args:     tools.Args{testKey: "1.2"},
			expected: float64Ptr(1.2),
		},
		{
			name:     "float64",
			args:     tools.Args{testKey: float64(1.2)},
			expected: float64Ptr(1.2),
		},
		{
			name:     "json_number_float",
			args:     tools.Args{testKey: json.Number("1.5")},
			expected: float64Ptr(1.5),
		},
		{
			name:     "json_number_int",
			args:     tools.Args{testKey: json.Number("1")},
			expected: float64Ptr(1.0),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			result := test.args.GetFloat64(testKey)

			assert.Equal(t, test.expected == nil, result == nil)

			if test.expected != nil && result != nil {
				assert.InDelta(t, *test.expected, *result, 0.01)
			}
		})
	}
}

func TestArgs_GetBool(t *testing.T) { //nolint:funlen
	t.Parallel()

	boolPtr := func(input bool) *bool {
		return &input
	}

	tests := []struct {
		name     string
		args     tools.Args
		expected *bool
	}{
		{
			name:     "nil",
			args:     nil,
			expected: nil,
		},
		{
			name:     "empty",
			args:     tools.Args{},
			expected: nil,
		},
		{
			name:     "int",
			args:     tools.Args{testKey: 1},
			expected: nil,
		},
		{
			name:     "float",
			args:     tools.Args{testKey: 1.2},
			expected: nil,
		},
		{
			name:     "garbage_str",
			args:     tools.Args{testKey: "garbage"},
			expected: nil,
		},
		{
			name:     "bool",
			args:     tools.Args{testKey: true},
			expected: boolPtr(true),
		},
		{
			name:     "bool_str",
			args:     tools.Args{testKey: "true"},
			expected: boolPtr(true),
		},
		{
			name:     "bool_str_int",
			args:     tools.Args{testKey: "1"},
			expected: boolPtr(true),
		},
		{
			name:     "bool_str_letter",
			args:     tools.Args{testKey: "f"},
			expected: boolPtr(false),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			result := test.args.GetBool(testKey)

			assert.Equal(t, test.expected == nil, result == nil)

			if test.expected != nil && result != nil {
				assert.Equal(t, *test.expected, *result)
			}
		})
	}
}

func TestArgs_GetStringSlice(t *testing.T) {
	t.Parallel()

	// TODO?
	// rawArgs := `{"test": ["1", "2", "3"]}`
	//
	// parsedArgs, err := tools.GetArgs([]byte(rawArgs), tools.Params{
	// 	{
	// 		Key:      testKey,
	// 		Type:     tools.ParamTypeArray,
	// 		ItemType: tools.ParamTypeString,
	// 	},
	// })
	// if err != nil {
	// 	t.Fatalf("failed to get args for test case: %v", err)
	// }

	tests := []struct {
		name     string
		args     tools.Args
		expected []string
	}{
		{
			name:     "nil",
			args:     nil,
			expected: nil,
		},
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
		// {
		// 	name:     "parsed_args",
		// 	args:     parsedArgs,
		// 	expected: []string{"1", "2", "3"},
		// },
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
