package tools

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/cneill/smoke/pkg/fs"
	"github.com/cneill/smoke/pkg/plan"
)

const (
	PlanAddTasks         = "tasks"
	PlanAddTasksContent  = "content"
	PlanAddTasksID       = "id"
	PlanAddTasksParentID = "parent_id"

	PlanAddContext        = "context"
	PlanAddContextType    = "type"
	PlanAddContextContent = "content"
	PlanAddContextOwners  = "owners"
)

type PlanAddTool struct {
	ProjectPath  string
	SessionName  string
	PlanFileName string
	PlanFile     *os.File
	PlanManager  *plan.Manager
}

// TODO: allow tool initializations to fail with error...?
func NewPlanAddTool(projectPath, sessionName string) Tool {
	planFileName := sessionName + "_plan.json"

	relPath, err := fs.GetRelativePath(projectPath, planFileName)
	if err != nil {
		panic(fmt.Errorf("invalid session plan file path (%s): %w", planFileName, err))
	}

	// TODO: stat for existing plan file, create the manager by loading if exists
	file, err := os.OpenFile(relPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		panic(fmt.Errorf("failed to open session plan file: %w", err))
	}

	manager := plan.NewManager(file)

	return &PlanAddTool{
		ProjectPath:  projectPath,
		SessionName:  sessionName,
		PlanFileName: planFileName,
		PlanManager:  manager,
	}
}

func (p *PlanAddTool) Name() string { return ToolPlanAdd }
func (p *PlanAddTool) Description() string {
	examples := CollectExamples(p.Examples()...)

	return "Add tasks and sub-tasks to the plan that will be executed during `work_process`." + examples
}

func (p *PlanAddTool) Examples() Examples {
	return Examples{
		{
			Description: "Add a few simple tasks and sub-tasks",
			Args: Args{
				PlanAddTasks: []Args{
					{
						PlanAddTasksID:      "DoThing_context",
						PlanAddTasksContent: "Update the DoThing() function to use context.Context",
					},
					{
						PlanAddTasksID:       "DoThing_context_param",
						PlanAddTasksContent:  "Update the DoThing() function to take a context.Context in its parameters",
						PlanAddTasksParentID: "DoThing_context",
					},
					{
						PlanAddTasksID:       "DoThing_context_cancellation",
						PlanAddTasksContent:  "Update the DoThing() function to use the provided context.Context for cancellation",
						PlanAddTasksParentID: "DoThing_context",
					},
				},
			},
		},
	}
}

func (p *PlanAddTool) Params() Params {
	return Params{
		{
			Key:         PlanAddTasks,
			Description: "An array of 1 or more tasks to be completed",
			Type:        ParamTypeArray,
			Required:    false,
			ItemType:    ParamTypeObject,
			NestedParams: Params{
				{
					Key: PlanAddTasksContent,
					Description: "The description of the task to be completed. This should be concise, but should " +
						"describe the task in sufficient detail to allow for implementation even if the conversation " +
						"is reset.",
					Type:     ParamTypeString,
					Required: true,
				},
				{
					Key: PlanAddTasksID,
					Description: fmt.Sprintf(
						"A short, unique identifier for this task. Can be used to link sub-tasks with %q",
						PlanAddTasksParentID),
					Type:     ParamTypeString,
					Required: true,
				},
				{
					Key:         PlanAddTasksParentID,
					Description: "If this is a sub-task of another task, provide the unique ID of its parent",
					Type:        ParamTypeString,
					Required:    false,
				},
			},
		},
		{
			Key:         PlanAddContext,
			Description: "An array of 1 or more items of context",
			Type:        ParamTypeArray,
			Required:    false,
			ItemType:    ParamTypeObject,
			NestedParams: Params{
				{
					Key: PlanAddContextContent,
					Description: "A piece of context you want to associate with one or more tasks in order to help " +
						"you stay on track and implement the user's request.",
					Type:     ParamTypeString,
					Required: true,
				},
				{
					Key: PlanAddContextType,
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
					Key:         PlanAddContextOwners,
					Description: "The IDs of the tasks for which this piece of context is relevant",
					Type:        ParamTypeArray,
					ItemType:    ParamTypeString,
					Required:    true,
				},
			},
		},
	}
}

func (p *PlanAddTool) Run(_ context.Context, args Args) (string, error) {
	tasks := args.GetArgsObjectSlice(PlanAddTasks)

	for taskIdx, task := range tasks {
		slog.Debug("Task details", "num", taskIdx, "task", task)
		id := task.GetString(PlanAddTasksID)
		content := task.GetString(PlanAddTasksContent)
		parentID := task.GetString(PlanAddTasksParentID)

		task := plan.NewTaskItem(*content).SetID(*id)
		if parentID != nil {
			task = task.SetParent(*parentID)
		}

		item := &plan.ItemUnion{TaskItem: task}
		if err := p.PlanManager.AddItem(item); err != nil {
			return "", fmt.Errorf("failed to add task with index %d: %w", taskIdx, err)
		}
	}

	context := args.GetArgsObjectSlice(PlanAddContext)

	for contextIdx, contextItem := range context {
		slog.Debug("Context details", "num", contextIdx, "context", contextItem)
		content := contextItem.GetString(PlanAddContextContent)
		rawContextType := contextItem.GetString(PlanAddContextType)
		owners := contextItem.GetStringSlice(PlanAddContextOwners)

		contextItem := plan.NewContextItem(plan.ContextType(*rawContextType), *content)
		contextItem.SetOwners(owners...)

		item := &plan.ItemUnion{ContextItem: contextItem}
		if err := p.PlanManager.AddItem(item); err != nil {
			return "", fmt.Errorf("failed to add context with index %d: %w", contextIdx, err)
		}
	}

	return "", nil
}
