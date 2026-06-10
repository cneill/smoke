package ask

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/cneill/smoke/pkg/ask"
	"github.com/cneill/smoke/pkg/tools"
)

const (
	ParamQuestion = "question"
	ParamOptions  = "options"
)

type Ask struct {
	manager *ask.Manager
}

func New(_, _ string) (tools.Tool, error) {
	return &Ask{}, nil
}

func (a *Ask) Name() string { return tools.NameAsk }

func (a *Ask) Description() string {
	examples := tools.CollectExamples(a.Examples()...)
	description := "Ask the user to choose one of several options or none of the above. " +
		"Returns structured JSON text with the selection, optional follow-up text, and cancellation state."

	return description + examples
}

func (a *Ask) Examples() tools.Examples {
	return tools.Examples{{
		Description: "Ask the user to choose a next step from a short list of options.",
		Args: tools.Args{
			ParamQuestion: "Which option should I take?",
			ParamOptions:  []string{"Refactor first", "Add tests first", "Do both"},
		},
	}}
}

func (a *Ask) Params() tools.Params {
	return tools.Params{
		{
			Key:         ParamQuestion,
			Description: "The question to ask the user",
			Type:        tools.ParamTypeString,
			Required:    true,
		},
		{
			Key:         ParamOptions,
			Description: "A list of 1 to 5 answer options",
			Type:        tools.ParamTypeArray,
			ItemType:    tools.ParamTypeString,
			Required:    true,
		},
	}
}

func (a *Ask) SetAskManager(manager *ask.Manager) {
	a.manager = manager
}

func (a *Ask) Run(ctx context.Context, args tools.Args) (*tools.Output, error) {
	if a.manager == nil {
		return nil, fmt.Errorf("ask manager not set")
	}

	question := args.GetString(ParamQuestion)
	if question == nil {
		return nil, fmt.Errorf("%w: missing question", tools.ErrArguments)
	}

	options := args.GetStringSlice(ParamOptions)

	responseChan, err := a.manager.Begin(*question, options)
	if err != nil {
		return nil, fmt.Errorf("failed to begin ask request: %w", err)
	}

	select {
	case <-ctx.Done():
		return nil, fmt.Errorf("ask canceled: %w", ctx.Err())
	case res := <-responseChan:
		payload, err := json.Marshal(res)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal ask result: %w", err)
		}

		return &tools.Output{Text: string(payload)}, nil
	}
}
