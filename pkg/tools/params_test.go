package tools_test

import (
	"testing"

	"github.com/cneill/smoke/pkg/tools"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParam_OK(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		param    tools.Param
		errorStr string
	}{
		{
			name:     "empty",
			param:    tools.Param{},
			errorStr: "missing key",
		},
		{
			name: "no_key",
			param: tools.Param{
				Description: "test",
				Type:        tools.ParamTypeArray,
			},
			errorStr: "missing key",
		},
		{
			name: "no_description",
			param: tools.Param{
				Key:  "test",
				Type: tools.ParamTypeArray,
			},
			errorStr: "missing description",
		},
		{
			name: "no_type",
			param: tools.Param{
				Key:         "test",
				Description: "test",
			},
			errorStr: "missing type",
		},
		{
			name: "invalid_type",
			param: tools.Param{
				Key:         "test",
				Description: "test",
				Type:        tools.ParamType("invalid"),
			},
			errorStr: `invalid param type: "invalid"`,
		},
		{
			name: "number_with_item_type",
			param: tools.Param{
				Key:         "test",
				Description: "test",
				Type:        tools.ParamTypeNumber,
				ItemType:    tools.ParamTypeString,
			},
			errorStr: "item type defined for non-array param type",
		},
		{
			name: "number_with_enum_strings",
			param: tools.Param{
				Key:              "test",
				Description:      "test",
				Type:             tools.ParamTypeNumber,
				EnumStringValues: []string{"test"},
			},
			errorStr: "string enum values defined for non-string param type",
		},
		{
			name: "number_with_nested_params",
			param: tools.Param{
				Key:         "test",
				Description: "test",
				Type:        tools.ParamTypeNumber,
				NestedParams: tools.Params{
					{
						Key:         "nested",
						Description: "nested",
						Type:        tools.ParamTypeString,
					},
				},
			},
			errorStr: "nested params defined for non-object param type",
		},
		{
			name: "nested_missing_description",
			param: tools.Param{
				Key:         "test",
				Description: "test",
				Type:        tools.ParamTypeObject,
				NestedParams: tools.Params{
					{
						Key:  "nested",
						Type: tools.ParamTypeString,
					},
				},
			},
			errorStr: `error with nested params: error with param at index 0 (key=nested): missing description`,
		},
		{
			name: "valid_with_nested_enum",
			param: tools.Param{
				Key:         "test",
				Description: "test",
				Type:        tools.ParamTypeObject,
				NestedParams: tools.Params{
					{
						Key:              "nested",
						Description:      "nested",
						Type:             tools.ParamTypeString,
						EnumStringValues: []string{"test"},
					},
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			err := test.param.OK()
			if test.errorStr != "" {
				require.EqualError(t, err, test.errorStr)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestParams_JSONSchemaProperties(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		params      tools.Params
		expected    map[string]any
		errorString string
	}{
		{
			name:     "nil",
			params:   nil,
			expected: map[string]any{},
		},
		{
			name: "invalid_nested",
			params: tools.Params{
				{
					Key:         "test",
					Description: "test",
					Type:        tools.ParamTypeObject,
					NestedParams: tools.Params{
						{
							Description:      "nested",
							Type:             tools.ParamTypeString,
							EnumStringValues: []string{"test"},
						},
					},
				},
			},
			expected: nil,
			errorString: "param validation error: error with param at index 0 (key=test): error with nested params: " +
				"error with param at index 0 (key=): missing key",
		},
		{
			name: "valid_nested_params",
			params: tools.Params{
				{
					Key:         "test",
					Description: "test",
					Type:        tools.ParamTypeObject,
					NestedParams: tools.Params{
						{
							Key:              "nested_enum",
							Description:      "nested_enum",
							Type:             tools.ParamTypeString,
							EnumStringValues: []string{"test"},
							Required:         true,
						},
						{
							Key:         "nested_array",
							Description: "nested_array",
							Type:        tools.ParamTypeArray,
							ItemType:    tools.ParamTypeNumber,
						},
						{
							Key:         "nested_object",
							Description: "nested_object",
							Type:        tools.ParamTypeObject,
							NestedParams: tools.Params{
								{
									Key:         "double_nested_number",
									Description: "double_nested_number",
									Type:        tools.ParamTypeNumber,
									Required:    true,
								},
							},
						},
					},
				},
			},
			expected: map[string]any{
				"test": map[string]any{
					"description": "test",
					"type":        tools.ParamTypeObject,
					"properties": map[string]any{
						"nested_enum": map[string]any{
							"description": "nested_enum",
							"type":        tools.ParamTypeString,
							"enum":        []string{"test"},
						},
						"nested_array": map[string]any{
							"description": "nested_array",
							"type":        tools.ParamTypeArray,
							"items": map[string]any{
								"type": tools.ParamTypeNumber,
							},
						},
						"nested_object": map[string]any{
							"description": "nested_object",
							"type":        tools.ParamTypeObject,
							"properties": map[string]any{
								"double_nested_number": map[string]any{
									"description": "double_nested_number",
									"type":        tools.ParamTypeNumber,
								},
							},
							"required": []string{"double_nested_number"},
						},
					},
					"required": []string{"nested_enum"},
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			result, err := test.params.JSONSchemaProperties()
			if test.errorString != "" {
				require.EqualError(t, err, test.errorString)
			} else {
				require.NoError(t, err)
			}

			assert.Equal(t, test.expected, result)
		})
	}
}
