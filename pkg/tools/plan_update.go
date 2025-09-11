package tools

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/cneill/smoke/pkg/plan"
)

const (
	PlanUpdateTasks         = "tasks"
	PlanUpdateTasksContent  = "content"
	PlanUpdateTasksID       = "id"
	PlanUpdateTasksParentID = "parent_id"

	PlanUpdateContext        = "context"
	PlanUpdateContextType    = "type"
	PlanUpdateContextContent = "content"
	PlanUpdateContextID      = "id"
	PlanUpdateContextOwners  = "owners"
)

type PlanUpdateTool struct {
	ProjectPath  string
	SessionName  string
	PlanFileName string
	PlanFile     *os.File
	PlanManager  *plan.Manager
}

func NewPlanUpdateTool(projectPath, sessionName string) Tool {
	return &PlanUpdateTool{
		ProjectPath: projectPath,
		SessionName: sessionName,
	}
}

func (p *PlanUpdateTool) Name() string { return ToolPlanUpdate }
func (p *PlanUpdateTool) Description() string {
	examples := CollectExamples(p.Examples()...)

	return "Update tasks and items of context that have already been added to the plan by referring to their IDs." +
		examples
}

func (p *PlanUpdateTool) SetPlanManager(manager *plan.Manager) { p.PlanManager = manager }

func (p *PlanUpdateTool) Examples() Examples {
	return Examples{
		{
			Description: "Update an existing task",
			Args: Args{
				PlanUpdateTasks: []Args{
					{
						PlanUpdateTasksID:      "DoThing_context",
						PlanUpdateTasksContent: "Update the DoThing() function to use context.Context instead of context.TODO",
					},
				},
			},
		},
	}
}

func (p *PlanUpdateTool) Params() Params {
	return Params{
		{
			Key:         PlanUpdateTasks,
			Description: "An array of 1 or more tasks to be completed",
			Type:        ParamTypeArray,
			Required:    false,
			ItemType:    ParamTypeObject,
			NestedParams: Params{
				{
					Key: PlanUpdateTasksContent,
					Description: "The description of the task to be completed. This should be concise, but should " +
						"describe the task in sufficient detail to allow for implementation even if the conversation " +
						"is reset.",
					Type:     ParamTypeString,
					Required: true,
				},
				{
					Key: PlanUpdateTasksID,
					Description: fmt.Sprintf(
						"A short, unique identifier for this task. Can be used to link sub-tasks with %q",
						PlanUpdateTasksParentID),
					Type:     ParamTypeString,
					Required: true,
				},
				{
					Key:         PlanUpdateTasksParentID,
					Description: "If this is a sub-task of another task, provide the unique ID of its parent",
					Type:        ParamTypeString,
					Required:    false,
				},
			},
		},
		{
			Key:         PlanUpdateContext,
			Description: "An array of 1 or more items of context",
			Type:        ParamTypeArray,
			Required:    false,
			ItemType:    ParamTypeObject,
			NestedParams: Params{
				{
					Key: PlanUpdateContextContent,
					Description: "A piece of context you want to associate with one or more tasks in order to help " +
						"you stay on track and implement the user's request.",
					Type:     ParamTypeString,
					Required: true,
				},
				{
					Key: PlanUpdateContextType,
					Description: fmt.Sprintf(
						"The type of context this represents. %q is a snippet or long section of source code from "+
							"e.g. the %q tool. %q is a decision made about the design of the solution that will be "+
							`developed as part of "work_process". %q is reference material about a 3rd party library`+
							"or external service.",
						plan.ContextTypeCode, ToolReadFile, plan.ContextTypeDecision, plan.ContextTypeReference),
					Type: ParamTypeString,
					EnumStringValues: ToStrings(
						[]plan.ContextType{plan.ContextTypeCode, plan.ContextTypeDecision, plan.ContextTypeReference}),
					Required: true,
				},
				{
					Key:         PlanUpdateContextID,
					Description: "A short, unique identifier for this piece of context.",
					Type:        ParamTypeString,
					Required:    true,
				},
				{
					Key:         PlanUpdateContextOwners,
					Description: "The IDs of the tasks for which this piece of context is relevant",
					Type:        ParamTypeArray,
					ItemType:    ParamTypeString,
					Required:    true,
				},
			},
		},
	}
}

func (p *PlanUpdateTool) Run(_ context.Context, args Args) (string, error) {
	if tasks := args.GetArgsObjectSlice(PlanUpdateTasks); tasks != nil {
		if err := p.handleTasks(tasks); err != nil {
			return "", err
		}
	}

	if context := args.GetArgsObjectSlice(PlanUpdateContext); context != nil {
		if err := p.handleContext(context); err != nil {
			return "", err
		}
	}

	return "", nil
}

func (p *PlanUpdateTool) handleTasks(tasks []Args) error {
	for taskIdx, rawTask := range tasks {
		slog.Debug("Task details", "num", taskIdx, "task", rawTask)
		id := rawTask.GetString(PlanUpdateTasksID)
		content := rawTask.GetString(PlanUpdateTasksContent)
		parentID := rawTask.GetString(PlanUpdateTasksParentID)

		task := plan.NewTaskItem(*content).SetID(*id)
		if parentID != nil {
			task = task.SetParent(*parentID)
		}

		item := &plan.ItemUnion{TaskItem: task}
		if err := p.PlanManager.AddItem(item); err != nil {
			return fmt.Errorf("failed to add task with index %d: %w", taskIdx, err)
		}
	}

	return nil
}

func (p *PlanUpdateTool) handleContext(context []Args) error {
	for contextIdx, rawContext := range context {
		slog.Debug("Context details", "num", contextIdx, "context", rawContext)
		rawContextType := rawContext.GetString(PlanUpdateContextType)
		content := rawContext.GetString(PlanUpdateContextContent)
		id := rawContext.GetString(PlanUpdateContextID)
		owners := rawContext.GetStringSlice(PlanUpdateContextOwners)

		contextItem := plan.NewContextItem(plan.ContextType(*rawContextType), *content)
		contextItem.SetOwners(owners...).SetID(*id)

		item := &plan.ItemUnion{ContextItem: contextItem}
		if err := p.PlanManager.AddItem(item); err != nil {
			return fmt.Errorf("failed to add context with index %d: %w", contextIdx, err)
		}
	}

	return nil
}
