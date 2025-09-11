package tools

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/cneill/smoke/pkg/plan"
)

const (
	PlanUpdateTasks             = "tasks"
	PlanUpdateTasksContent      = "content"
	PlanUpdateTasksID           = "id"
	PlanUpdateTasksParentID     = "parent_id"
	PlanUpdateTasksDependencies = "dependencies"

	PlanUpdateContext        = "context"
	PlanUpdateContextType    = "type"
	PlanUpdateContextContent = "content"
	PlanUpdateContextID      = "id"
	PlanUpdateContextOwners  = "owners"
)

type PlanUpdateTool struct {
	ProjectPath string
	SessionName string
	PlanManager *plan.Manager
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
			Description: "An array of 1 or more existing tasks to be updated",
			Type:        ParamTypeArray,
			Required:    false,
			ItemType:    ParamTypeObject,
			NestedParams: Params{
				{
					Key:         PlanUpdateTasksID,
					Description: "The identifier for an existing task or sub-task.",
					Type:        ParamTypeString,
					Required:    true,
				},
				{
					Key: PlanUpdateTasksContent,
					Description: "Used to adjust the description of the referenced task. This should be concise, but " +
						"should describe the task in sufficient detail to allow for implementation even if the " +
						"conversation is reset.",
					Type:     ParamTypeString,
					Required: false,
				},
				{
					Key:         PlanUpdateTasksParentID,
					Description: "Adjust the parent of a task or sub-task by providing the new parent's ID.",
					Type:        ParamTypeString,
					Required:    false,
				},
				{
					Key: PlanUpdateTasksDependencies,
					Description: "Update the list of depenendencies on the referenced task or sub-task by providing " +
						"a full array of the new dependencies.",
					Type:     ParamTypeArray,
					ItemType: ParamTypeString,
					Required: false,
				},
			},
		},
		{
			Key:         PlanUpdateContext,
			Description: "An array of 1 or more existing items of context to be updated",
			Type:        ParamTypeArray,
			Required:    false,
			ItemType:    ParamTypeObject,
			NestedParams: Params{
				{
					Key:         PlanUpdateContextID,
					Description: "The identifier for an existing piece of context.",
					Type:        ParamTypeString,
					Required:    true,
				},
				{
					Key:         PlanUpdateContextContent,
					Description: "Used to update the content of the referenced piece of context.",
					Type:        ParamTypeString,
					Required:    false,
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
					Required: false,
				},
				{
					Key: PlanUpdateContextOwners,
					Description: "Adjust the owners, or tasks for which this piece of context is relevant, by " +
						"providing their IDs.",
					Type:     ParamTypeArray,
					ItemType: ParamTypeString,
					Required: false,
				},
			},
		},
	}
}

func (p *PlanUpdateTool) Run(_ context.Context, args Args) (string, error) {
	var taskIDs, contextIDs []string
	var err error

	tasks := args.GetArgsObjectSlice(PlanUpdateTasks)
	if tasks != nil {
		taskIDs, err = p.handleTasks(tasks)
		if err != nil {
			return "", err
		}
	}

	context := args.GetArgsObjectSlice(PlanUpdateContext)
	if context != nil {
		contextIDs, err = p.handleContext(context)
		if err != nil {
			return "", err
		}
	}

	if len(tasks) == 0 && len(context) == 0 {
		return "", fmt.Errorf("no tasks or context items were provided to update")
	}

	output := "Updated "

	if len(taskIDs) > 0 {
		output += fmt.Sprintf("tasks with IDs %s; ", strings.Join(taskIDs, ", "))
	}

	if len(contextIDs) > 0 {
		output += fmt.Sprintf("context items with IDs %s; ", strings.Join(taskIDs, ", "))
	}

	output = strings.TrimRight(output, "; ")

	return output, nil
}

func (p *PlanUpdateTool) handleTasks(tasks []Args) ([]string, error) {
	taskIDs := make([]string, len(tasks))

	for taskIdx, rawTask := range tasks {
		slog.Debug("Updating task details", "num", taskIdx, "task", rawTask)
		id := rawTask.GetString(PlanUpdateTasksID)

		existing := p.PlanManager.GetItemByID(*id)
		if existing == nil {
			return []string{}, fmt.Errorf("no existing task with the ID %q was found to update", *id)
		}

		content := rawTask.GetString(PlanUpdateTasksContent)
		if content == nil {
			content = &existing.TaskItem.Content
		}

		parentID := rawTask.GetString(PlanUpdateTasksParentID)
		if parentID == nil {
			parentID = &existing.TaskItem.Parent
		}

		dependencies := rawTask.GetStringSlice(PlanAddTasksDependencies)
		if dependencies == nil {
			dependencies = append([]string{}, existing.TaskItem.Dependencies...)
		}

		task := plan.NewTaskItem(*content).
			SetID(*id).
			SetOperation(plan.OperationUpdate).
			SetParent(*parentID).
			SetDependencies(dependencies...)

		item := &plan.ItemUnion{TaskItem: task}
		if err := p.PlanManager.HandleItem(item); err != nil {
			return []string{}, fmt.Errorf("failed to add task with index %d: %w", taskIdx, err)
		}

		taskIDs[taskIdx] = *id
	}

	return taskIDs, nil
}

func (p *PlanUpdateTool) handleContext(context []Args) ([]string, error) {
	contextIDs := make([]string, len(context))

	for contextIdx, rawContext := range context {
		slog.Debug("Updating context details", "num", contextIdx, "context", rawContext)

		id := rawContext.GetString(PlanUpdateContextID)

		existing := p.PlanManager.GetItemByID(*id)
		if existing == nil {
			return []string{}, fmt.Errorf("no existing context item with the ID %q was found to update", *id)
		}

		contextType := rawContext.GetString(PlanUpdateContextType)
		if contextType == nil {
			existingType := string(existing.ContextItem.ContextType)
			contextType = &existingType
		}

		content := rawContext.GetString(PlanUpdateContextContent)
		if content == nil {
			content = &existing.ContextItem.Content
		}

		owners := rawContext.GetStringSlice(PlanUpdateContextOwners)
		if owners == nil {
			owners = append([]string{}, existing.ContextItem.Owners...)
		}

		contextItem := plan.NewContextItem(plan.ContextType(*contextType), *content).
			SetID(*id).
			SetOperation(plan.OperationUpdate).
			SetOwners(owners...)

		item := &plan.ItemUnion{ContextItem: contextItem}
		if err := p.PlanManager.HandleItem(item); err != nil {
			return []string{}, fmt.Errorf("failed to add context with index %d: %w", contextIdx, err)
		}

		contextIDs[contextIdx] = *id
	}

	return contextIDs, nil
}
