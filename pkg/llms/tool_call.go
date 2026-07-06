package llms

import (
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/cneill/smoke/pkg/tools"
)

type ToolCall struct {
	ID   string     `json:"id"`
	Name string     `json:"name"`
	Args tools.Args `json:"args"`

	// RawArgs contains the raw provider-supplied tool arguments. It is used to replay assistant tool calls even when
	// Args could not be parsed or validated.
	RawArgs string `json:"raw_args,omitempty"`
	// ArgsError contains an argument parse or validation error that prevented this call from being safely executed.
	ArgsError string `json:"args_error,omitempty"`
}

func (t ToolCall) ArgsString() string {
	if t.RawArgs != "" {
		return t.RawArgs
	}

	if t.Args != nil {
		return t.Args.String()
	}

	return "{}"
}

func (t ToolCall) ProviderArgs() any {
	if t.Args != nil {
		return t.Args
	}

	var raw any
	if err := json.Unmarshal([]byte(t.RawArgs), &raw); err != nil {
		return t.RawArgs
	}

	return raw
}

func (t ToolCall) InvalidArgs() bool {
	return t.ArgsError != ""
}

func (t ToolCall) GetArgsErr() error {
	if t.ArgsError == "" {
		return nil
	}

	return fmt.Errorf("%s", t.ArgsError)
}

func (t ToolCall) LogValue() slog.Value {
	attrs := []slog.Attr{
		slog.String("ID", t.ID),
		slog.String("Name", t.Name),
		slog.Any("Args", t.Args),
	}

	if t.ArgsError != "" {
		attrs = append(attrs, []slog.Attr{
			slog.String("RawArgs", t.RawArgs),
			slog.String("ArgsError", t.ArgsError),
		}...)
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
