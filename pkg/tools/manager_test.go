package tools_test

import (
	"context"
	"encoding/json"
	"io"
	"testing"

	"github.com/cneill/smoke/pkg/tools"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type dummyTool struct {
	params tools.Params
}

const dummyName = "dummy"

func (d dummyTool) Name() string                                             { return dummyName }
func (d dummyTool) Description() string                                      { return "dummy tool for testing" }
func (d dummyTool) Examples() tools.Examples                                 { return nil }
func (d dummyTool) Params() tools.Params                                     { return d.params }
func (d dummyTool) Run(ctx context.Context, args tools.Args) (string, error) { return "", nil }

func getManager(t *testing.T, params tools.Params) *tools.Manager {
	t.Helper()

	manager := tools.NewManager(".", "test")
	dummy := dummyTool{params: params}
	manager.SetTools(func(_, _ string) tools.Tool {
		return dummy
	})

	return manager
}

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

			manager := getManager(t, test.params)
			args, err := manager.GetArgs(dummyName, []byte(test.input))
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
