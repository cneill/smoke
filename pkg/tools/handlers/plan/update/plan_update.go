package planupdate

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/cneill/smoke/pkg/plan"
	"github.com/cneill/smoke/pkg/tools"
)

const (
	ParamTasks             = "tasks"
	ParamTasksContent      = "content"
	ParamTasksID           = "id"
	ParamTasksParentID     = "parent_id"
	ParamTasksDependencies = "dependencies"

	ParamContext        = "context"
	ParamContextType    = "type"
	ParamContextContent = "content"
	ParamContextID      = "id"
	ParamContextOwners  = "owners"
)

type PlanUpdate struct {
	ProjectPath string
	SessionName string
	PlanManager *plan.Manager
}

func New(projectPath, sessionName string) (tools.Tool, error) {
	return &PlanUpdate{
		ProjectPath: projectPath,
		SessionName: sessionName,
	}, nil
}

func (p *PlanUpdate) Name() string { return tools.NamePlanUpdate }
func (p *PlanUpdate) Description() string {
	examples := tools.CollectExamples(p.Examples()...)

	return "Update tasks and items of context that have already been added to the plan by referring to their IDs." +
		examples
}

func (p *PlanUpdate) SetPlanManager(manager *plan.Manager) { p.PlanManager = manager }

func (p *PlanUpdate) Examples() tools.Examples {
	return tools.Examples{
		{
			Description: "Update an existing task",
			Args: tools.Args{
				ParamTasks: []tools.Args{
					{
						ParamTasksID:      "DoThing_context",
						ParamTasksContent: "Update the DoThing() function to use context.Context instead of context.TODO",
					},
				},
			},
		},
		{
			Description: "Update task dependencies after discovering additional requirements",
			Args: tools.Args{
				ParamTasks: []tools.Args{
					{
						ParamTasksID: "deploy_service",
						ParamTasksDependencies: []string{
							"create_dockerfile", "setup_kubernetes", "configure_monitoring",
						},
					},
				},
			},
		},
		{
			Description: "Update context items including changing type and adding/removing owners",
			Args: tools.Args{
				ParamContext: []tools.Args{
					{
						ParamContextID:   "api_design_notes",
						ParamContextType: plan.ContextTypeDecision,
						ParamContextContent: "After team discussion, decided to use REST API instead of GraphQL " +
							"for better caching and simpler client implementation",
						ParamContextOwners: []string{"implement_api", "design_api_schema", "create_api_docs"},
					},
				},
			},
		},
		{
			Description: "Move a subtask to a different parent task during reorganization",
			Args: tools.Args{
				ParamTasks: []tools.Args{
					{
						ParamTasksID:       "validate_user_input",
						ParamTasksParentID: "user_service_layer",
						ParamTasksContent: "Move input validation from the controller layer to the service " +
							"layer for better reusability",
					},
				},
			},
		},
	}
}

func (p *PlanUpdate) Params() tools.Params { //nolint:funlen
	return tools.Params{
		{
			Key:         ParamTasks,
			Description: "An array of 1 or more existing tasks to be updated",
			Type:        tools.ParamTypeArray,
			Required:    false,
			ItemType:    tools.ParamTypeObject,
			NestedParams: tools.Params{
				{
					Key:         ParamTasksID,
					Description: "The identifier for an existing task or sub-task.",
					Type:        tools.ParamTypeString,
					Required:    true,
				},
				{
					Key: ParamTasksContent,
					Description: "Used to adjust the description of the referenced task. This should be concise, but " +
						"should describe the task in sufficient detail to allow for implementation even if the " +
						"conversation is reset.",
					Type:     tools.ParamTypeString,
					Required: false,
				},
				{
					Key:         ParamTasksParentID,
					Description: "Adjust the parent of a task or sub-task by providing the new parent's ID.",
					Type:        tools.ParamTypeString,
					Required:    false,
				},
				{
					Key: ParamTasksDependencies,
					Description: "Update the list of depenendencies on the referenced task or sub-task by providing " +
						"a full array of the new dependencies.",
					Type:     tools.ParamTypeArray,
					ItemType: tools.ParamTypeString,
					Required: false,
				},
			},
		},
		{
			Key:         ParamContext,
			Description: "An array of 1 or more existing items of context to be updated",
			Type:        tools.ParamTypeArray,
			Required:    false,
			ItemType:    tools.ParamTypeObject,
			NestedParams: tools.Params{
				{
					Key:         ParamContextID,
					Description: "The identifier for an existing piece of context.",
					Type:        tools.ParamTypeString,
					Required:    true,
				},
				{
					Key:         ParamContextContent,
					Description: "Used to update the content of the referenced piece of context.",
					Type:        tools.ParamTypeString,
					Required:    false,
				},
				{
					Key: ParamContextType,
					Description: fmt.Sprintf(
						"The type of context this represents. %q is a snippet or long section of source code from "+
							"e.g. the %q tool. %q is a decision made about the design of the solution that will be "+
							"developed as part of `work_process`. %q is reference material about a 3rd party library"+
							"or external service. %q is a constraint imposed either by the user or the underlying "+
							"codebase. %q is something that is commonly used by other similar parts of the code.",
						plan.ContextTypeCode, tools.NameReadFile, plan.ContextTypeDecision, plan.ContextTypeReference,
						plan.ContextTypeConstraint, plan.ContextTypeConvention),
					Type: tools.ParamTypeString,
					EnumStringValues: tools.ToStrings([]plan.ContextType{
						plan.ContextTypeCode, plan.ContextTypeConstraint, plan.ContextTypeDecision,
						plan.ContextTypeReference,
					}),
					Required: false,
				},

				{
					Key: ParamContextOwners,
					Description: "Adjust the owners, or tasks for which this piece of context is relevant, by " +
						"providing their IDs.",
					Type:     tools.ParamTypeArray,
					ItemType: tools.ParamTypeString,
					Required: false,
				},
			},
		},
	}
}

func (p *PlanUpdate) Run(_ context.Context, args tools.Args) (string, error) {
	var (
		taskIDs, contextIDs []string
		err                 error
	)

	tasks := args.GetArgsObjectSlice(ParamTasks)
	if tasks != nil {
		taskIDs, err = p.handleTasks(tasks)
		if err != nil {
			return "", err
		}
	}

	context := args.GetArgsObjectSlice(ParamContext)
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
		output += fmt.Sprintf("context items with IDs %s; ", strings.Join(contextIDs, ", "))
	}

	output = strings.TrimRight(output, "; ")

	// TODO: maybe include current context with ReadPlanTool ?
	return output, nil
}

func (p *PlanUpdate) handleTasks(tasks []tools.Args) ([]string, error) {
	taskIDs := make([]string, len(tasks))

	for taskIdx, rawTask := range tasks {
		slog.Debug("Updating task details", "num", taskIdx, "task", rawTask)
		id := rawTask.GetString(ParamTasksID)

		existing := p.PlanManager.GetItemByID(*id)
		if existing == nil {
			return []string{}, fmt.Errorf("no existing task with the ID %q was found to update", *id)
		}

		content := rawTask.GetString(ParamTasksContent)
		if content == nil {
			content = &existing.TaskItem.Content
		}

		parentID := rawTask.GetString(ParamTasksParentID)
		if parentID == nil {
			parentID = &existing.TaskItem.Parent
		}

		dependencies := rawTask.GetStringSlice(ParamTasksDependencies)
		if dependencies == nil {
			dependencies = append([]string{}, existing.TaskItem.Dependencies...)
		}

		task := plan.NewTaskItem(*id, *content, plan.OperationUpdate).
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

func (p *PlanUpdate) handleContext(context []tools.Args) ([]string, error) {
	contextIDs := make([]string, len(context))

	for contextIdx, rawContext := range context {
		slog.Debug("Updating context details", "num", contextIdx, "context", rawContext)

		id := rawContext.GetString(ParamContextID)

		existing := p.PlanManager.GetItemByID(*id)
		if existing == nil {
			return []string{}, fmt.Errorf("no existing context item with the ID %q was found to update", *id)
		}

		contextType := rawContext.GetString(ParamContextType)
		if contextType == nil {
			existingType := string(existing.ContextItem.ContextType)
			contextType = &existingType
		}

		content := rawContext.GetString(ParamContextContent)
		if content == nil {
			content = &existing.ContextItem.Content
		}

		owners := rawContext.GetStringSlice(ParamContextOwners)
		if owners == nil {
			owners = append([]string{}, existing.ContextItem.Owners...)
		}

		contextItem := plan.NewContextItem(plan.ContextType(*contextType), *content, plan.OperationUpdate).
			SetID(*id).
			SetOwners(owners...)

		item := &plan.ItemUnion{ContextItem: contextItem}
		if err := p.PlanManager.HandleItem(item); err != nil {
			return []string{}, fmt.Errorf("failed to add context with index %d: %w", contextIdx, err)
		}

		contextIDs[contextIdx] = *id
	}

	return contextIDs, nil
}
