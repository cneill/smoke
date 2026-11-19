package planread

import (
	"context"
	"fmt"

	"github.com/cneill/smoke/pkg/plan"
	"github.com/cneill/smoke/pkg/tools"
)

const (
	ParamPending   = "pending"
	ParamCompleted = "completed"
)

type PlanRead struct {
	ProjectPath string
	SessionName string
	PlanManager *plan.Manager
}

func New(projectPath, sessionName string) (tools.Tool, error) {
	return &PlanRead{
		ProjectPath: projectPath,
		SessionName: sessionName,
	}, nil
}

func (p *PlanRead) Name() string { return tools.NamePlanRead }
func (p *PlanRead) Description() string {
	examples := tools.CollectExamples(p.Examples()...)

	return fmt.Sprintf("Read the current state of the plan, including all tasks, sub-tasks, dependencies, completions, "+
		"and contexts. Must set at least one of %q / %q to true.%s", ParamPending, ParamCompleted, examples)
}

func (p *PlanRead) SetPlanManager(manager *plan.Manager) { p.PlanManager = manager }

func (p *PlanRead) Examples() tools.Examples {
	return tools.Examples{
		{
			Description: "Read only the tasks that have not been completed in the current plan",
			Args: tools.Args{
				ParamPending: true,
			},
		},
		{
			Description: "Read all pending and completed tasks in the current plan",
			Args: tools.Args{
				ParamPending:   true,
				ParamCompleted: true,
			},
		},
	}
}

func (p *PlanRead) Params() tools.Params {
	return tools.Params{
		{
			Key:         ParamPending,
			Description: "Include pending/incomplete plan items?",
			Type:        tools.ParamTypeBoolean,
		},
		{
			Key:         ParamCompleted,
			Description: "Include completed plan items?",
			Type:        tools.ParamTypeBoolean,
		},
	}
}

func (p *PlanRead) Run(_ context.Context, args tools.Args) (*tools.Output, error) { //nolint:cyclop
	if p.PlanManager == nil {
		return nil, fmt.Errorf("plan manager not set")
	}

	var pending, completed bool

	if pendingRaw := args.GetBool(ParamPending); pendingRaw != nil && *pendingRaw {
		pending = true
	}

	if completedRaw := args.GetBool(ParamCompleted); completedRaw != nil && *completedRaw {
		completed = true
	}

	if !pending && !completed {
		return nil, fmt.Errorf("%w: must set at least one of %q / %q to true", tools.ErrArguments, ParamPending, ParamCompleted)
	}

	content := p.PlanManager.Render(plan.RenderOpts{
		Pending:   pending,
		Completed: completed,
	})
	if content == "" {
		switch {
		case pending && !completed:
			content = "No pending plan items."
		case completed && !pending:
			content = "No completed plan items."
		case completed && pending:
			content = "No existing plan items."
		}
	}

	output := &tools.Output{
		Text: content,
	}

	return output, nil
}
