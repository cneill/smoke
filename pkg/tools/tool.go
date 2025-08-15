// Package tools contains the [tools.Tool] interface defintion, as well as individual Tool implementations such as
// [ReadFileTool]. These are used by an LLM to take actions on the codebase.
package tools

import "fmt"

type Tool interface {
	Name() string
	Description() string
	Params() Params
	Run(args Args) (string, error)
}

type Tools []Tool

func (t Tools) Params(name string) (Params, error) {
	for _, tool := range t {
		if tool.Name() == name {
			return tool.Params(), nil
		}
	}

	return Params{}, ErrUnknownTool
}

func (t Tools) Call(name string, args Args) (string, error) {
	for _, tool := range t {
		if tool.Name() == name {
			output, err := tool.Run(args)
			if err != nil {
				return "", fmt.Errorf("%w: %w", ErrCallFailed, err)
			}

			return output, nil
		}
	}

	return "", ErrUnknownTool
}
