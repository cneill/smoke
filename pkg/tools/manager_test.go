package tools_test

import (
	"context"
	"encoding/json"
	"io"
	"path/filepath"
	"testing"

	"github.com/cneill/smoke/pkg/plan"
	"github.com/cneill/smoke/pkg/tools"
	"github.com/cneill/smoke/pkg/tools/handlers"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type dummyTool struct {
	params tools.Params
}

const dummyName = "dummy"

func (d dummyTool) Name() string                                        { return dummyName }
func (d dummyTool) Description() string                                 { return "dummy tool for testing" }
func (d dummyTool) Examples() tools.Examples                            { return nil }
func (d dummyTool) Params() tools.Params                                { return d.params }
func (d dummyTool) Run(_ context.Context, _ tools.Args) (string, error) { return "", nil }

func getManager(t *testing.T, params tools.Params) *tools.Manager {
	t.Helper()

	absPath, err := filepath.Abs(".")
	require.NoError(t, err)

	planFilePath := filepath.Join(t.TempDir(), "plan_file.json")
	planManager, err := plan.ManagerFromPath(planFilePath)
	require.NoError(t, err)

	opts := &tools.ManagerOpts{
		ProjectPath:      absPath,
		SessionName:      "test",
		ToolInitializers: handlers.AllTools(),
		PlanManager:      planManager,
		TeaEmitter:       func(tea.Msg) {},
	}

	manager, err := tools.NewManager(opts)
	require.NoError(t, err)

	dummy := dummyTool{params: params}

	manager.InitTools(func(_, _ string) (tools.Tool, error) {
		return dummy, nil
	})

	return manager
}

func TestGetArgs(t *testing.T) { //nolint:funlen,maintidx
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
			name:  "invalid_enum_value",
			input: `{"key": "wrong_value"}`,
			params: tools.Params{
				{
					Key:              "key",
					Description:      "key",
					Type:             tools.ParamTypeString,
					Required:         true,
					EnumStringValues: []string{"right_value"},
				},
			},
			expected: nil,
			errors:   []error{tools.ErrUnexpectedValue},
		},
		{
			name:  "valid_enum_values",
			input: `{"key": "right_value", "key2": "test"}`,
			params: tools.Params{
				{
					Key:              "key",
					Description:      "key",
					Type:             tools.ParamTypeString,
					Required:         true,
					EnumStringValues: []string{"right_value"},
				},
				{
					Key:              "key2",
					Description:      "key2",
					Type:             tools.ParamTypeString,
					Required:         true,
					EnumStringValues: []string{"test"},
				},
			},
			expected: tools.Args{"key": "right_value", "key2": "test"},
			errors:   nil,
		},
		{
			name:  "invalid_object_param",
			input: `{"object": {"property2": 1}}`,
			params: tools.Params{
				{
					Key:         "object",
					Description: "object_param",
					Type:        tools.ParamTypeObject,
					Required:    true,
					NestedParams: tools.Params{
						{
							Key:         "property1",
							Description: "property1",
							Type:        tools.ParamTypeNumber,
							Required:    true,
						},
					},
				},
			},
			expected: nil,
			errors:   []error{tools.ErrUnknownKeys, tools.ErrUnexpectedValue},
		},
		{
			name: "multiple_valid",
			input: `{"number_key_int": 1, "number_key_float": 2.0, "str_key": "test", "bool_key": false, ` +
				`"object_key": {}, "str_array_key": ["a", "b", "c"], "int_array_key": [1, 2, 3], ` +
				`"string_enum_key": "good3"}`,
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
					Description: "int slice key",
					Type:        tools.ParamTypeArray,
					ItemType:    tools.ParamTypeNumber,
					Required:    true,
				},
				// TODO: figure out why this isn't being evaluated for right/wrong???
				{
					Key:              "string_enum_key",
					Description:      "string enum key",
					Type:             tools.ParamTypeString,
					Required:         true,
					EnumStringValues: []string{"good1", "good2", "good3"},
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
				"string_enum_key":  "good3",
			},
			errors: nil,
		},
		{
			name:  "valid_object_param",
			input: `{"object": {"property1": 1}}`,
			params: tools.Params{
				{
					Key:         "object",
					Description: "object_param",
					Type:        tools.ParamTypeObject,
					Required:    true,
					NestedParams: tools.Params{
						{
							Key:         "property1",
							Description: "property1",
							Type:        tools.ParamTypeNumber,
							Required:    true,
						},
					},
				},
			},
			expected: tools.Args{
				"object": map[string]any{
					"property1": json.Number("1"),
				},
			},
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
