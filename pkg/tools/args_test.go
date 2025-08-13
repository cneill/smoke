package tools_test

import (
	"encoding/json"
	"io"
	"testing"
	"time"

	"github.com/cneill/smoke/pkg/tools"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testKey = "test"

func TestGetArgs(t *testing.T) { //nolint:funlen
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		params   tools.Params
		expected tools.Args
		errors   []error
	}{
		{
			name:     "empty_input",
			input:    "",
			params:   tools.Params{},
			expected: nil,
			errors:   []error{tools.ErrInvalidJSON, io.EOF},
		},
		{
			name:     "invalid_json",
			input:    `{"a":`,
			params:   tools.Params{},
			expected: nil,
			errors:   []error{tools.ErrInvalidJSON},
		},
		{
			name:     "unknown_key",
			input:    `{"unknown": "1"}`,
			params:   tools.Params{},
			expected: nil,
			errors:   []error{tools.ErrUnknownKeys},
		},
		{
			name:  "missing_required_key",
			input: `{}`,
			params: tools.Params{
				{
					Key:         "missing",
					Description: "missing key",
					Type:        tools.ParamTypeBoolean,
					Required:    true,
				},
			},
			expected: nil,
			errors:   []error{tools.ErrMissingKeys},
		},
		{
			name:  "missing_optional_key",
			input: `{}`,
			params: tools.Params{
				{
					Key:         "missing",
					Description: "missing key",
					Type:        tools.ParamTypeBoolean,
					Required:    false,
				},
			},
			expected: tools.Args{},
			errors:   nil,
		},
		{
			name:  "wrong_type_required_bool_key",
			input: `{"wrong_type": 1.0}`,
			params: tools.Params{
				{
					Key:         "wrong_type",
					Description: "wrong type key",
					Type:        tools.ParamTypeBoolean,
					Required:    true,
				},
			},
			expected: nil,
			errors:   []error{tools.ErrWrongTypeKeys},
		},
		{
			name:  "wrong_type_optional_bool_key",
			input: `{"wrong_type": 1.0}`,
			params: tools.Params{
				{
					Key:         "wrong_type",
					Description: "wrong type key",
					Type:        tools.ParamTypeBoolean,
					Required:    false,
				},
			},
			expected: nil,
			errors:   []error{tools.ErrWrongTypeKeys},
		},
		{
			name:  "wrong_type_number_key",
			input: `{"wrong_type": "abc"}`,
			params: tools.Params{
				{
					Key:         "wrong_type",
					Description: "wrong type key",
					Type:        tools.ParamTypeNumber,
					Required:    true,
				},
			},
			expected: nil,
			errors:   []error{tools.ErrWrongTypeKeys},
		},
		{
			name:  "wrong_item_type_string",
			input: `{"wrong_type": [1, 2, 3]}`,
			params: tools.Params{
				{
					Key:         "wrong_type",
					Description: "wrong type key",
					Type:        tools.ParamTypeArray,
					ItemType:    tools.ParamTypeString,
					Required:    true,
				},
			},
			expected: nil,
			errors:   []error{tools.ErrWrongTypeKeys},
		},
		{
			name:  "wrong_item_type_number",
			input: `{"wrong_type": ["1", "2", "3"]}`,
			params: tools.Params{
				{
					Key:         "wrong_type",
					Description: "wrong type key",
					Type:        tools.ParamTypeArray,
					ItemType:    tools.ParamTypeNumber,
					Required:    true,
				},
			},
			expected: nil,
			errors:   []error{tools.ErrWrongTypeKeys},
		},
		{
			name: "multiple_valid",
			input: `{"number_key_int": 1, "number_key_float": 2.0, "str_key": "test", "bool_key": false, ` +
				`"object_key": {}, "str_array_key": ["a", "b", "c"], "int_array_key": [1, 2, 3]}`,
			params: tools.Params{
				{
					Key:         "number_key_int",
					Description: "int key",
					Type:        tools.ParamTypeNumber,
					Required:    true,
				},
				{
					Key:         "number_key_float",
					Description: "float key",
					Type:        tools.ParamTypeNumber,
					Required:    true,
				},
				{
					Key:         "str_key",
					Description: "str key",
					Type:        tools.ParamTypeString,
					Required:    true,
				},
				{
					Key:         "bool_key",
					Description: "bool key",
					Type:        tools.ParamTypeBoolean,
					Required:    true,
				},
				{
					Key:         "object_key",
					Description: "object key",
					Type:        tools.ParamTypeObject,
					Required:    true,
				},
				{
					Key:         "str_array_key",
					Description: "string slice key",
					Type:        tools.ParamTypeArray,
					ItemType:    tools.ParamTypeString,
					Required:    true,
				},
				{
					Key:         "int_array_key",
					Description: "string slice key",
					Type:        tools.ParamTypeArray,
					ItemType:    tools.ParamTypeNumber,
					Required:    true,
				},
			},
			expected: tools.Args{
				"number_key_int":   json.Number("1"),
				"number_key_float": json.Number("2.0"),
				"str_key":          "test",
				"bool_key":         false,
				"object_key":       map[string]any{},
				"str_array_key":    []any{"a", "b", "c"},
				"int_array_key":    []any{json.Number("1"), json.Number("2"), json.Number("3")},
			},
			errors: nil,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			args, err := tools.GetArgs([]byte(test.input), test.params)
			if test.errors == nil {
				require.NoError(t, err)
			} else {
				for _, testErr := range test.errors {
					require.ErrorIs(t, err, testErr)
				}
			}

			assert.Equal(t, test.expected, args)
		})
	}
}

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

func TestArgs_GetInt64(t *testing.T) { //nolint:funlen
	t.Parallel()

	int64Ptr := func(input int64) *int64 {
		return &input
	}

	tests := []struct {
		name     string
		args     tools.Args
		expected *int64
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
			name:     "float_str",
			args:     tools.Args{testKey: "1.2"},
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
			expected: int64Ptr(1),
		},
		{
			name:     "int64",
			args:     tools.Args{testKey: int64(1)},
			expected: int64Ptr(1),
		},
		{
			name:     "string",
			args:     tools.Args{testKey: "1"},
			expected: int64Ptr(1),
		},
		{
			name:     "json_number_int",
			args:     tools.Args{testKey: json.Number("1")},
			expected: int64Ptr(1),
		},
		{
			name:     "json_number_float",
			args:     tools.Args{testKey: json.Number("1.5")},
			expected: nil,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			result := test.args.GetInt64(testKey)

			assert.Equal(t, test.expected == nil, result == nil)

			if test.expected != nil && result != nil {
				assert.Equal(t, *test.expected, *result)
			}
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

func TestArgs_GetStringSlice(t *testing.T) { //nolint:funlen
	t.Parallel()

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
