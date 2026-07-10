package mcp //nolint:testpackage

import (
	"testing"

	"github.com/cneill/smoke/pkg/tools"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParamsFromRawSchemaHandlesNullableArrayProperties(t *testing.T) {
	t.Parallel()

	rawSchema := []byte(`{
		"type": "object",
		"properties": {
			"files": {
				"type": ["null", "array"],
				"items": {
					"type": "string"
				},
				"description": "absolute paths to active files, if any"
			},
			"packagePaths": {
				"type": ["null", "array"],
				"items": {
					"type": "string"
				},
				"description": "the go package paths to describe"
			}
		},
		"required": ["packagePaths"],
		"additionalProperties": false
	}`)

	params, err := paramsFromRawSchema(rawSchema)
	require.NoError(t, err)

	require.Len(t, params, 2)
	assert.Equal(t, tools.Param{
		Key:         "files",
		Description: "absolute paths to active files, if any",
		Type:        tools.ParamTypeArray,
		Nullable:    true,
		ItemType:    tools.ParamTypeString,
	}, params[0])
	assert.Equal(t, tools.Param{
		Key:         "packagePaths",
		Description: "the go package paths to describe",
		Type:        tools.ParamTypeArray,
		Nullable:    true,
		Required:    true,
		ItemType:    tools.ParamTypeString,
	}, params[1])
}

func TestNullableMCPParamsProduceValidJSONSchemaProperties(t *testing.T) {
	t.Parallel()

	params := tools.Params{
		{
			Key:         "files",
			Description: "absolute paths to active files, if any",
			Type:        tools.ParamTypeArray,
			Nullable:    true,
			ItemType:    tools.ParamTypeString,
		},
	}

	properties, err := params.JSONSchemaProperties()
	require.NoError(t, err)

	assert.Equal(t, map[string]any{
		"files": map[string]any{
			"type":        []tools.ParamType{tools.ParamTypeNull, tools.ParamTypeArray},
			"description": "absolute paths to active files, if any",
			"items": map[string]any{
				"type": tools.ParamTypeString,
			},
		},
	}, properties)
}
