package tools

import "fmt"

// ParamType maps to JSON Schema types for properties. See here:
// https://json-schema.org/understanding-json-schema/reference/type
type ParamType string

const (
	ParamTypeArray   ParamType = "array"
	ParamTypeBoolean ParamType = "boolean"
	ParamTypeNull    ParamType = "null"
	ParamTypeNumber  ParamType = "number"
	ParamTypeObject  ParamType = "object"
	ParamTypeString  ParamType = "string"
)

// Param holds details about a specific parameter that an LLM can provide when making a tool call.
type Param struct {
	// Key is the name of the JSON object key that should be supplied.
	Key string
	// Description explains what this Param does.
	Description string
	// Type corresponds to one of the JSON data types.
	Type ParamType
	// Required says whether this Param must be supplied to execute the associated [Tool].
	Required bool
	// ItemType corresponds to the type of individual items if Type is ParamTypeArray.
	ItemType ParamType
	// EnumStringValues contains an optional list of acceptable string values for the field.
	EnumStringValues []string
	// NestedParams is for Objects with nested properties.
	NestedParams Params
}

func (p Param) OK() error {
	switch {
	case p.Key == "":
		return fmt.Errorf("missing key")
	case p.Description == "":
		return fmt.Errorf("missing description")
	case p.Type == "":
		return fmt.Errorf("missing type")
	case p.ItemType != "" && p.Type != ParamTypeArray:
		return fmt.Errorf("item type specified for non-array param type")
	case len(p.EnumStringValues) > 0 && p.Type != ParamTypeString:
		return fmt.Errorf("string enum values supplied for non-string param type")
	case len(p.NestedParams) > 0 && p.Type != ParamTypeObject:
		return fmt.Errorf("nested properties supplied for non-string param type")
	}

	return nil
}

// Params is a convenience type for a slice of Param structs.
type Params []Param

func (p Params) ByKey(key string) *Param {
	for _, param := range p {
		if param.Key == key {
			return &param
		}
	}

	return nil
}

// Keys returns a slice of all keys for the Params in the slice.
func (p Params) Keys() []string {
	results := []string{}
	for _, param := range p {
		results = append(results, param.Key)
	}

	return results
}

// RequiredKeys returns a slice of keys for all Params with Required=true.
func (p Params) RequiredKeys() []string {
	results := []string{}

	for _, param := range p {
		if param.Required {
			results = append(results, param.Key)
		}
	}

	return results
}

// Required checks whether the given 'key' 1) exists, and 2) is marked with Required=true.
func (p Params) Required(key string) bool {
	for _, param := range p {
		if param.Key == key && param.Required {
			return true
		}
	}

	return false
}

// JSONSchemaProperties returns the value to be used in the "properties" key of an object's JSON Schema definition.
// Generally used for Tool definitions.
func (p Params) JSONSchemaProperties() (map[string]any, error) {
	properties := map[string]any{}

	for paramIdx, param := range p {
		if err := param.OK(); err != nil {
			return nil, fmt.Errorf("param %d (key=%q) was invalid: %w", paramIdx, param.Key, err)
		}

		keyProps := map[string]any{
			"type":        param.Type,
			"description": param.Description,
		}

		if param.ItemType != "" {
			keyProps["items"] = map[string]any{
				"type": param.ItemType,
			}
		}

		if len(param.EnumStringValues) > 0 {
			keyProps["enum"] = param.EnumStringValues
		}

		if param.ItemType == ParamTypeObject && len(param.NestedParams) > 0 {
			nestedProps, err := param.NestedParams.JSONSchemaProperties()
			if err != nil {
				return nil, fmt.Errorf("error with nested properties on param %d (key=%q): %w", paramIdx, param.Key, err)
			}

			keyProps["properties"] = nestedProps
			keyProps["required"] = param.NestedParams.RequiredKeys()
		}

		properties[param.Key] = keyProps
	}

	return properties, nil
}
