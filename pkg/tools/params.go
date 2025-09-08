package tools

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
