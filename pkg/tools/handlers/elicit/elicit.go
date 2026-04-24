package elicit

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/cneill/smoke/pkg/elicit"
	"github.com/cneill/smoke/pkg/tools"
)

const (
	ParamQuestion = "question"
	ParamOptions  = "options"
)

type Elicit struct {
	runtime *elicit.Runtime
}

func New(_, _ string) (tools.Tool, error) {
	return &Elicit{}, nil
}

func (e *Elicit) Name() string { return tools.NameElicit }

func (e *Elicit) Description() string {
	examples := tools.CollectExamples(e.Examples()...)
	description := "Ask the user to choose one of several options or none of the above. " +
		"Returns structured JSON text with the selection, optional follow-up text, and cancellation state."

	return description + examples
}

func (e *Elicit) Examples() tools.Examples {
	return tools.Examples{{
		Description: "Ask the user to choose a next step from a short list of options.",
		Args: tools.Args{
			ParamQuestion: "Which option should I take?",
			ParamOptions:  []string{"Refactor first", "Add tests first", "Do both"},
		},
	}}
}

func (e *Elicit) Params() tools.Params {
	return tools.Params{
		{Key: ParamQuestion, Description: "The question to ask the user", Type: tools.ParamTypeString, Required: true},
		{
			Key:         ParamOptions,
			Description: "A list of 1 to 5 answer options",
			Type:        tools.ParamTypeArray,
			ItemType:    tools.ParamTypeString, Required: true,
		},
	}
}

func (e *Elicit) SetElicitRuntime(runtime *elicit.Runtime) {
	e.runtime = runtime
}

func (e *Elicit) Run(ctx context.Context, args tools.Args) (*tools.Output, error) {
	if e.runtime == nil {
		return nil, fmt.Errorf("elicit runtime not set")
	}

	question := args.GetString(ParamQuestion)
	if question == nil || *question == "" {
		return nil, fmt.Errorf("%w: missing question", tools.ErrArguments)
	}

	options := args.GetStringSlice(ParamOptions)
	if options == nil {
		return nil, fmt.Errorf("%w: missing options", tools.ErrArguments)
	}

	if len(options) == 0 || len(options) > 5 {
		return nil, fmt.Errorf("%w: options must contain between 1 and 5 items", tools.ErrArguments)
	}

	resultChan, err := e.runtime.Begin(elicit.Request{Question: *question, Options: options})
	if err != nil {
		return nil, fmt.Errorf("failed to begin elicit request: %w", err)
	}

	select {
	case <-ctx.Done():
		return nil, fmt.Errorf("elicit canceled: %w", ctx.Err())
	case res := <-resultChan:
		payload, err := json.Marshal(res)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal elicit result: %w", err)
		}

		return &tools.Output{Text: string(payload)}, nil
	}
}
