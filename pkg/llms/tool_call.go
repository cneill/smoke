package llms

import "github.com/cneill/smoke/pkg/tools"

type ToolCall struct {
	ID   string     `json:"id"`
	Name string     `json:"name"`
	Args tools.Args `json:"args"`
}

type ToolCalls []ToolCall

func (t ToolCalls) Clone() ToolCalls {
	cloned := make(ToolCalls, len(t))
	copy(cloned, t)

	return cloned
}

func (t ToolCalls) Names() []string {
	names := make([]string, len(t))
	for i, call := range t {
		names[i] = call.Name
	}

	return names
}
