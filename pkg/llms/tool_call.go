package llms

import "github.com/cneill/smoke/pkg/tools"

type ToolCall struct {
	ID   string
	Name string
	Args tools.Args
}

type ToolCalls []ToolCall

func (t ToolCalls) Clone() ToolCalls {
	// TODO: actually clone!!!
	return t
}

func (t ToolCalls) Names() []string {
	names := make([]string, len(t))
	for _, call := range t {
		names = append(names, call.Name)
	}

	return names
}
