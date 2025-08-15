// Package tools contains the [Tool] interface definition, as well as individual Tool implementations such as
// [ReadFileTool]. These are used by an LLM to take actions on the codebase.
package tools

import "fmt"

type Tool interface {
	// Name returns the name by which an LLM will reference the tool.
	Name() string
	// Description returns a string explaining how to use the tool.
	Description() string
	// Params returns the set of parameters that a tool can accept, including type information and whether they are
	// required for the tool to execute.
	Params() Params
	// Run executes the tool and returns a string that gets returned to the LLM, or an error.
	Run(args Args) (string, error)
}

// Tools is a convenience type for working with a set of individual [Tool] structs.
type Tools []Tool

// Params returns the [Params] slice for the [Tool] with the provided 'name'.
func (t Tools) Params(name string) (Params, error) {
	for _, tool := range t {
		if tool.Name() == name {
			return tool.Params(), nil
		}
	}

	return Params{}, ErrUnknownTool
}

// Call finds the [Tool] with 'name' (if present in the slice) and calls it with the provided 'args'. If no [Tool]
// matches 'name', it returns [ErrUnknownTool]. Otherwise it returns the output, or the error returned by the Run
// method wrapped with [ErrCallFailed].
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
