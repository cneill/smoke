package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/cneill/smoke/pkg/plan"
)

const (
	PlanCompletionTaskIDs = "task_ids"
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

	return "Mark tasks as completed by ID." + examples
}

func (p *PlanCompletionTool) SetPlanManager(manager *plan.Manager) { p.PlanManager = manager }

func (p *PlanCompletionTool) Examples() Examples {
	return Examples{
		{
			Description: "Mark a single task as completed with default success status",
			Args: Args{
				PlanCompletionTaskIDs: []string{"task_id_1"},
			},
		},
		{
			Description: "Mark multiple tasks as completed with failed status",
			Args: Args{
				PlanCompletionTaskIDs: []string{"task_id_1", "task_id_2"},
				PlanCompletionStatus:  string(plan.CompletionStatusFailed),
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
			Key: PlanCompletionStatus,
			Description: fmt.Sprintf(
				"The completion status for the tasks. If not provided, defaults to %q.",
				plan.CompletionStatusSuccess,
			),
			Type:     ParamTypeString,
			Required: false,
			EnumStringValues: ToStrings(
				[]plan.CompletionStatus{plan.CompletionStatusSuccess, plan.CompletionStatusFailed, plan.CompletionStatusPartial}),
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

	completionItem := plan.NewCompletionItem(taskIDs...).
		SetStatus(status)

	item := &plan.ItemUnion{CompletionItem: completionItem}
	if err := p.PlanManager.HandleItem(item); err != nil {
		return "", fmt.Errorf("failed to mark tasks as completed: %w", err)
	}

	return "Marked the following tasks as completed: " + strings.Join(taskIDs, ", "), nil
}
