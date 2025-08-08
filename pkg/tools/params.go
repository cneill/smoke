package tools

type ParamType string

const (
	ParamTypeArray   ParamType = "array"
	ParamTypeBoolean ParamType = "boolean"
	ParamTypeNull    ParamType = "null"
	ParamTypeNumber  ParamType = "number"
	ParamTypeObject  ParamType = "object"
	ParamTypeString  ParamType = "string"
)

type Param struct {
	Key         string
	Description string
	Type        ParamType
	Required    bool
	ItemType    ParamType
}

type Params []Param

func (p Params) ByKey(key string) *Param {
	for _, param := range p {
		if param.Key == key {
			return &param
		}
	}

	return nil
}

func (p Params) Keys() []string {
	results := []string{}
	for _, param := range p {
		results = append(results, param.Key)
	}

	return results
}

func (p Params) RequiredKeys() []string {
	results := []string{}

	for _, param := range p {
		if param.Required {
			results = append(results, param.Key)
		}
	}

	return results
}

func (p Params) Required(key string) bool {
	for _, param := range p {
		if param.Key == key && param.Required {
			return true
		}
	}

	return false
}
