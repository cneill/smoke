// Package tools contains the [Tool] interface definition, as well as individual Tool implementations such as
// [ReadFileTool]. These are used by an LLM to take actions on the codebase.
package tools

import "context"

type Tool interface {
	// Name returns the name by which an LLM will reference the tool.
	Name() string
	// Description returns a string explaining how to use the tool.
	Description() string
	// Examples to be included in the description to explain how to use the tool.
	Examples() Examples
	// Params returns the set of parameters that a tool can accept, including type information and whether they are
	// required for the tool to execute.
	Params() Params
	// Run executes the tool and returns a string that gets returned to the LLM, or an error.
	Run(ctx context.Context, args Args) (string, error)
}

type Tools []Tool

func (t Tools) Names() []string {
	results := []string{}

	for _, tool := range t {
		results = append(results, tool.Name())
	}

	return results
}

// TODO: make initializer more general; this isn't really ideal
type Initializer func(projectPath, sessionName string) Tool
