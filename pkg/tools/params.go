package tools

import (
	"fmt"
	"slices"
)

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
	// NestedParams is for Objects or Arrays of Objects with nested properties.
	NestedParams Params
}

func (p Param) OK() error { //nolint:cyclop
	switch {
	case p.Key == "":
		return fmt.Errorf("missing key")
	case p.Description == "":
		return fmt.Errorf("missing description")
	case p.Type == "":
		return fmt.Errorf("missing type")
	case !slices.Contains(
		[]ParamType{ParamTypeArray, ParamTypeBoolean, ParamTypeNull, ParamTypeNumber, ParamTypeObject, ParamTypeString}, p.Type):
		return fmt.Errorf("invalid param type: %q", p.Type)
	case p.ItemType != "" && p.Type != ParamTypeArray:
		return fmt.Errorf("item type defined for non-array param type")
	case len(p.EnumStringValues) > 0 && p.Type != ParamTypeString:
		return fmt.Errorf("string enum values defined for non-string param type")
	case len(p.NestedParams) > 0:
		if p.Type != ParamTypeObject && (p.Type != ParamTypeArray || p.ItemType != ParamTypeObject) {
			return fmt.Errorf("nested params defined for non-object param type, or array without object items")
		}

		if err := p.NestedParams.OK(); err != nil {
			return fmt.Errorf("error with nested params: %w", err)
		}
	}

	return nil
}

// JSONSchemaProperties returns the properties for a single Param, or an error if invalid. Typically used by the method
// of the same name on the [Params] type.
func (p Param) JSONSchemaProperties() (map[string]any, error) {
	keyProps := map[string]any{
		"type":        p.Type,
		"description": p.Description,
	}

	// Specify the type of "items" in the array
	if p.Type == ParamTypeArray {
		keyProps["items"] = map[string]any{
			"type": p.ItemType,
		}
	}

	// Specify the valid enum values for a string field
	if len(p.EnumStringValues) > 0 {
		keyProps["enum"] = p.EnumStringValues
	}

	// Handle NestedParams for object params or array params containing objects
	if err := p.handleNestedParams(keyProps); err != nil {
		return nil, fmt.Errorf("failed to handle nested properties: %w", err)
	}

	return keyProps, nil
}

func (p Param) handleNestedParams(keyProps map[string]any) error {
	if len(p.NestedParams) == 0 {
		return nil
	}

	nestedProps, err := p.NestedParams.JSONSchemaProperties()
	if err != nil {
		return err
	}

	switch {
	case p.Type == ParamTypeObject:
		keyProps["properties"] = nestedProps
		keyProps["required"] = p.NestedParams.RequiredKeys()
	case p.Type == ParamTypeArray && p.ItemType == ParamTypeObject:
		itemsProps, ok := keyProps["items"].(map[string]any)
		if !ok {
			return fmt.Errorf(`failed to handle param for array of objects: no "items" key`)
		}

		itemsProps["properties"] = nestedProps
		itemsProps["required"] = p.NestedParams.RequiredKeys()

		keyProps["items"] = itemsProps
	}

	return nil
}

// Params is a convenience type for a slice of Param structs.
type Params []Param

func (p Params) OK() error {
	for paramIdx, param := range p {
		if err := param.OK(); err != nil {
			return fmt.Errorf("error with param at index %d (key=%s): %w", paramIdx, param.Key, err)
		}
	}

	return nil
}

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

	if err := p.OK(); err != nil {
		return nil, fmt.Errorf("param validation error: %w", err)
	}

	for paramIdx, param := range p {
		props, err := param.JSONSchemaProperties()
		if err != nil {
			return nil, fmt.Errorf("error with param at index %d (key=%q): %w", paramIdx, param.Key, err)
		}

		properties[param.Key] = props
	}

	return properties, nil
}
