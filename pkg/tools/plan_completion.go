package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/cneill/smoke/pkg/plan"
)

const (
	PlanCompletionTaskIDs = "task_ids"
	PlanCompletionContent = "content"
	PlanCompletionStatus  = "status"
)

type PlanCompletionTool struct {
	ProjectPath string
	SessionName string
	PlanManager *plan.Manager
}

func NewPlanCompletionTool(projectPath, sessionName string) Tool {
	return &PlanCompletionTool{
		ProjectPath: projectPath,
		SessionName: sessionName,
	}
}

func (p *PlanCompletionTool) Name() string { return ToolPlanCompletion }
func (p *PlanCompletionTool) Description() string {
	examples := CollectExamples(p.Examples()...)

	return fmt.Sprintf("Mark tasks as completed by their IDs, with a note explaining the status of these tasks. If "+
		"you need to completely redefine or remove a task or set of tasks based on user feedback, you can mark them "+
		"as %q to make it clear that these tasks should be ignored during `work_process`.%s",
		plan.CompletionStatusObsolete, examples,
	)
}

func (p *PlanCompletionTool) SetPlanManager(manager *plan.Manager) { p.PlanManager = manager }

func (p *PlanCompletionTool) Examples() Examples {
	return Examples{
		{
			Description: "Mark a single task as completed with default success status.",
			Args: Args{
				PlanCompletionTaskIDs: []string{"task_id_1"},
				PlanCompletionContent: "The Foo() function now returns an error, all call sites are updated to " +
					"account for this, and the relevant tests pass.",
			},
		},
		{
			Description: "Mark multiple tasks as completed with failed status, explaining that the user has failed " +
				"to provide all the necessary context to complete the task.",
			Args: Args{
				PlanCompletionTaskIDs: []string{"task_id_1", "task_id_2"},
				PlanCompletionContent: "The tests are failing after making modifications to the Foo() function, but " +
					"this appears to be due to a missing API key that I don't have access to.",
				PlanCompletionStatus: string(plan.CompletionStatusFailed),
			},
		},
		{
			Description: fmt.Sprintf("Mark a task and its child subtasks as obsolete after user feedback on the "+
				"plan. This is essentially equivalent to deleting the task and will ensure that it does not show up "+
				"in the output of %q.",
				ToolPlanRead,
			),
			Args: Args{
				PlanCompletionTaskIDs: []string{"task_id_1"},
				PlanCompletionContent: "The user explained that this task is outside the scope of the current " +
					"request and should not be implemented yet.",
				PlanCompletionStatus: string(plan.CompletionStatusObsolete),
			},
		},
	}
}

func (p *PlanCompletionTool) Params() Params {
	return Params{
		{
			Key:         PlanCompletionTaskIDs,
			Description: "The IDs of the tasks to mark as completed",
			Type:        ParamTypeArray,
			ItemType:    ParamTypeString,
			Required:    true,
		},
		{
			Key: PlanCompletionContent,
			Description: "A brief explanation of why the task is being marked completed, and why this particular " +
				"completion status was chosen.",
			Type:     ParamTypeString,
			Required: true,
		},
		{
			Key: PlanCompletionStatus,
			Description: fmt.Sprintf(
				"The completion status for the tasks. If not provided, defaults to %q. %q means that the task is "+
					"totally complete based on the user's query. %q means that it is impossible to complete any of "+
					"the task with the information available and it won't be attempted. %q means that some of the "+
					"task was completed, but the remainder cannot be completed due to missing information or "+
					"unexplained errors. %q means that the user has explicitly asked for this not to be implemented, "+
					"or there is a new version of the task that invalidates the old one and its context entirely.",
				plan.CompletionStatusSuccess, plan.CompletionStatusSuccess, plan.CompletionStatusFailed,
				plan.CompletionStatusPartial, plan.CompletionStatusObsolete,
			),
			Type:     ParamTypeString,
			Required: false,
			EnumStringValues: ToStrings([]plan.CompletionStatus{
				plan.CompletionStatusSuccess, plan.CompletionStatusFailed, plan.CompletionStatusPartial,
				plan.CompletionStatusObsolete,
			}),
		},
	}
}

func (p *PlanCompletionTool) Run(_ context.Context, args Args) (string, error) {
	if p.PlanManager == nil {
		return "", fmt.Errorf("plan manager not set")
	}

	taskIDs := args.GetStringSlice(PlanCompletionTaskIDs)
	if taskIDs == nil {
		return "", fmt.Errorf("no task IDs provided")
	}

	content := args.GetString(PlanCompletionContent)

	status := plan.CompletionStatusSuccess

	if statusStr := args.GetString(PlanCompletionStatus); statusStr != nil {
		status = plan.CompletionStatus(*statusStr)
	}

	// Validate that all provided IDs correspond to existing tasks
	for _, taskID := range taskIDs {
		item := p.PlanManager.GetItemByID(taskID)
		if item == nil || item.Type() != plan.ItemTypeTask {
			return "", fmt.Errorf("no task found with ID %q", taskID)
		}
	}

	completionItem := plan.NewCompletionItem(*content, taskIDs...).
		SetStatus(status)

	item := &plan.ItemUnion{CompletionItem: completionItem}
	if err := p.PlanManager.HandleItem(item); err != nil {
		return "", fmt.Errorf("failed to mark tasks as completed: %w", err)
	}

	return "Marked the following tasks as completed: " + strings.Join(taskIDs, ", "), nil
}
