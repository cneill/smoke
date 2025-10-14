// Package tools contains the [Tool] interface definition, as well as individual Tool implementations such as
// [ReadFileTool]. These are used by an LLM to take actions on the codebase.
package tools

import (
	"context"

	"github.com/cneill/smoke/pkg/plan"
	"github.com/google/jsonschema-go/jsonschema"
)

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

type PlanTool interface {
	Tool

	// SetPlanManager provides the plan.Manager to a tool that needs it to interact with the plan file.
	SetPlanManager(manager *plan.Manager)
}

type MCPTool interface {
	Tool

	// Source returns the name of the MCP client from which this Tool derives.
	Source() string
	// Schema returns the JSON Schema for the tool's parameters.
	Schema() *jsonschema.Schema
}

func AllTools() []Initializer {
	return []Initializer{
		// NewCreateDirectoryTool,
		// NewDDGTool,
		// NewGitDiffTool,
		// NewGoASTTool,
		// NewGoFumptTool,
		// NewGoImportsTool,
		// NewGoLintTool,
		// NewGoTestTool,
		// NewGrepTool,
		// NewListFilesTool,
		// NewPlanAddTool,
		// NewPlanCompletionTool,
		// NewPlanReadTool,
		// NewPlanUpdateTool,
		NewReadFileTool,
		NewReplaceLinesTool,
		// NewSummarizeHistoryTool,
		// NewWriteFileTool,
	}
}

func PlanningTools() []Initializer {
	return []Initializer{
		// NewDDGTool,
		// NewGitDiffTool,
		// NewGoASTTool,
		// NewGoLintTool,
		// NewGoTestTool,
		// NewGrepTool,
		// NewListFilesTool,
		// NewPlanAddTool,
		// NewPlanCompletionTool,
		// NewPlanReadTool,
		// NewPlanUpdateTool,
		NewReadFileTool,
		// NewSummarizeHistoryTool,
	}
}

func SummarizeTools() []Initializer {
	return []Initializer{
		// NewPlanReadTool,
	}
}
