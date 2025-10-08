package llms

import "github.com/cneill/smoke/pkg/tools"

type ToolCall struct {
	ID   string
	Name string
	Args tools.Args
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
