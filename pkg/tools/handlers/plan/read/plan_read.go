package planread

import (
	"context"
	"fmt"

	"github.com/cneill/smoke/pkg/plan"
	"github.com/cneill/smoke/pkg/tools"
)

const (
	Name = "plan_read"
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

func (p *PlanRead) Name() string { return Name }
func (p *PlanRead) Description() string {
	examples := tools.CollectExamples(p.Examples()...)

	return "Read the current state of the plan, including all tasks, sub-tasks, dependencies, completions, and " +
		"contexts." + examples
}

func (p *PlanRead) SetPlanManager(manager *plan.Manager) { p.PlanManager = manager }

func (p *PlanRead) Examples() tools.Examples {
	return tools.Examples{
		{
			Description: "Read the full current state of the plan\n\nThe output format shows:\n" +
				"- Tasks in a hierarchical tree structure with indentation for subtasks\n" +
				"- Task format: '- <content> (id: <id>) [<status>]'\n" +
				"- Status can be: pending, success, failed, partial, or obsolete\n" +
				"- Dependencies listed under tasks that have them\n" +
				"- Context items associated with each task\n" +
				"- All contexts listed at the end with their types and owners",
			Args: tools.Args{},
		},
	}
}

func (p *PlanRead) Params() tools.Params {
	return tools.Params{}
}

func (p *PlanRead) Run(_ context.Context, _ tools.Args) (string, error) {
	if p.PlanManager == nil {
		return "", fmt.Errorf("plan manager not set")
	}

	return p.PlanManager.Markdown(), nil
}
