package llms

import (
	"log/slog"

	"github.com/cneill/smoke/pkg/tools"
)

type ToolCall struct {
	ID   string     `json:"id"`
	Name string     `json:"name"`
	Args tools.Args `json:"args"`
}

func (t ToolCall) LogValue() slog.Value {
	attrs := []slog.Attr{
		slog.String("ID", t.ID),
		slog.String("Name", t.Name),
		slog.Any("Args", t.Args),
	}

	return slog.GroupValue(attrs...)
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
