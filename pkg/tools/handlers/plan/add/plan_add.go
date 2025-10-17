package planadd

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/cneill/smoke/pkg/plan"
	"github.com/cneill/smoke/pkg/tools"
)

const (
	Name = "plan_add"

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

type PlanAdd struct {
	ProjectPath string
	SessionName string
	PlanManager *plan.Manager
}

func New(projectPath, sessionName string) (tools.Tool, error) {
	return &PlanAdd{
		ProjectPath: projectPath,
		SessionName: sessionName,
	}, nil
}

func (p *PlanAdd) Name() string { return Name }
func (p *PlanAdd) Description() string {
	examples := tools.CollectExamples(p.Examples()...)

	return "Add tasks and subtasks to the plan during `plan_process` that will be executed during `work_process`, " +
		"or pieces of context linked to those tasks that are relevant to completing them. If your plan will require " +
		"editing multiple functions/methods extensively, create subtasks for each. If you are implementing simple " +
		"getter/setter methods, you can consolidate to a single, clear task description to cover them all." + examples
}

func (p *PlanAdd) SetPlanManager(manager *plan.Manager) { p.PlanManager = manager }

func (p *PlanAdd) Examples() tools.Examples { //nolint:funlen
	return tools.Examples{
		{
			Description: "Add a few simple tasks and sub-tasks",
			Args: tools.Args{
				ParamTasks: []tools.Args{
					{
						ParamTasksID:      "DoThing_context",
						ParamTasksContent: "Update the DoThing() function to use context.Context",
					},
					{
						ParamTasksID:       "DoThing_context_param",
						ParamTasksContent:  "Update the DoThing() function to take a context.Context in its parameters",
						ParamTasksParentID: "DoThing_context",
					},
					{
						ParamTasksID:       "DoThing_context_cancellation",
						ParamTasksContent:  "Update the DoThing() function to use the provided context.Context for cancellation",
						ParamTasksParentID: "DoThing_context",
					},
				},
			},
		},
		{
			Description: "Add some context about the existing codebase to make it clearer what work needs to be done " +
				"to complete the task.",
			Args: tools.Args{
				ParamTasks: []tools.Args{
					{
						ParamTasksID: "vendor_error_handling",
						ParamTasksContent: "Improve the error-handling for the VendorClient struct in the vendor " +
							"package by using named errors to make it more testable and consistent.",
					},
				},
				ParamContext: []tools.Args{
					{
						ParamContextID: "vendor_defined_errors",
						ParamContextContent: "The vendor package contains the ErrTimeout, ErrConnectionRefused, " +
							"ErrUnauthorized, and ErrUnknown error types.",
						ParamContextOwners: "vendor_error_handling",
						ParamContextType:   plan.ContextTypeCode,
					},
					{
						ParamContextID: "vendor_undefined_errors",
						ParamContextContent: "VendorClient methods return several identical error strings in " +
							"multiple locations that do not have corresponding types.",
						ParamContextOwners: "vendor_error_handling",
						ParamContextType:   plan.ContextTypeCode,
					},
					{
						ParamContextID: "vendor_api_details",
						ParamContextContent: "The package comment at the top of `pkg/vendor/vendor.go` includes a " +
							"list of possible errors returned by the vendor REST API, as defined in their docs.",
						ParamContextOwners: "vendor_error_handling",
						ParamContextType:   plan.ContextTypeReference,
					},
					{
						ParamContextID: "implementation_constraints",
						ParamContextContent: "Do not create named errors for every rare, one-off error. Focus on " +
							"naming errors that can occur in multiple different places.",
						ParamContextOwners: "vendor_error_handling",
						ParamContextType:   plan.ContextTypeConstraint,
					},
				},
			},
		},
		{
			Description: "Add tasks with dependencies to ensure proper execution order",
			Args: tools.Args{
				ParamTasks: []tools.Args{
					{
						ParamTasksID:      "database_migration",
						ParamTasksContent: "Create database migration scripts for the new user table schema",
					},
					{
						ParamTasksID:           "user_model",
						ParamTasksContent:      "Implement the User model struct with validation methods",
						ParamTasksDependencies: []string{"database_migration"},
					},
					{
						ParamTasksID:           "user_repository",
						ParamTasksContent:      "Create UserRepository with CRUD operations",
						ParamTasksDependencies: []string{"user_model", "database_migration"},
					},
					{
						ParamTasksID:           "user_service",
						ParamTasksContent:      "Implement UserService business logic layer",
						ParamTasksDependencies: []string{"user_repository"},
					},
				},
			},
		},
		{
			Description: "Add a complex multi-level task hierarchy for refactoring a large module",
			Args: tools.Args{
				ParamTasks: []tools.Args{
					{
						ParamTasksID:      "refactor_auth_module",
						ParamTasksContent: "Refactor the entire authentication module to use JWT tokens instead of sessions",
					},
					{
						ParamTasksID:       "refactor_auth_backend",
						ParamTasksContent:  "Update backend authentication logic",
						ParamTasksParentID: "refactor_auth_module",
					},
					{
						ParamTasksID:       "implement_jwt_generation",
						ParamTasksContent:  "Create JWT token generation and signing logic",
						ParamTasksParentID: "refactor_auth_backend",
					},
					{
						ParamTasksID:       "implement_jwt_validation",
						ParamTasksContent:  "Create JWT token validation and parsing logic",
						ParamTasksParentID: "refactor_auth_backend",
					},
					{
						ParamTasksID:           "update_auth_middleware",
						ParamTasksContent:      "Update authentication middleware to use JWT tokens",
						ParamTasksParentID:     "refactor_auth_backend",
						ParamTasksDependencies: []string{"implement_jwt_generation", "implement_jwt_validation"},
					},
					{
						ParamTasksID:       "refactor_auth_frontend",
						ParamTasksContent:  "Update frontend authentication flow",
						ParamTasksParentID: "refactor_auth_module",
					},
					{
						ParamTasksID:       "update_login_component",
						ParamTasksContent:  "Modify login component to handle JWT tokens",
						ParamTasksParentID: "refactor_auth_frontend",
					},
					{
						ParamTasksID:       "update_auth_service",
						ParamTasksContent:  "Update frontend auth service to store and send JWT tokens",
						ParamTasksParentID: "refactor_auth_frontend",
					},
				},
			},
		},
		{
			Description: "Add a task with all four types of context to demonstrate comprehensive planning",
			Args: tools.Args{
				ParamTasks: []tools.Args{
					{
						ParamTasksID:      "implement_rate_limiting",
						ParamTasksContent: "Add rate limiting to the API endpoints to prevent abuse",
					},
				},
				ParamContext: []tools.Args{
					{
						ParamContextID: "existing_middleware_code",
						ParamContextContent: "The current middleware stack is defined in pkg/middleware/chain.go " +
							"and uses the standard net/http Handler interface",
						ParamContextOwners: []string{"implement_rate_limiting"},
						ParamContextType:   plan.ContextTypeCode,
					},
					{
						ParamContextID: "rate_limit_algorithm_decision",
						ParamContextContent: "Use a sliding window algorithm with Redis backend for distributed " +
							"rate limiting across multiple server instances",
						ParamContextOwners: []string{"implement_rate_limiting"},
						ParamContextType:   plan.ContextTypeDecision,
					},
					{
						ParamContextID: "redis_rate_limiter_reference",
						ParamContextContent: "The go-redis/redis_rate library (github.com/go-redis/redis_rate/v10) " +
							"provides a proven implementation of sliding window rate limiting with Redis",
						ParamContextOwners: []string{"implement_rate_limiting"},
						ParamContextType:   plan.ContextTypeReference,
					},
					{
						ParamContextID: "rate_limit_constraints",
						ParamContextContent: "Rate limits must be configurable per endpoint, with default of 100 " +
							"requests per minute for authenticated users and 20 for anonymous users",
						ParamContextOwners: []string{"implement_rate_limiting"},
						ParamContextType:   plan.ContextTypeConstraint,
					},
				},
			},
		},
	}
}

func (p *PlanAdd) Params() tools.Params { //nolint:funlen
	return tools.Params{
		{
			Key:         ParamTasks,
			Description: "An array of 1 or more tasks to be completed",
			Type:        tools.ParamTypeArray,
			Required:    false,
			ItemType:    tools.ParamTypeObject,
			NestedParams: tools.Params{
				{
					Key: ParamTasksContent,
					Description: "The description of the task to be completed. This should be concise, but should " +
						"describe the task in sufficient detail to allow for implementation even if the conversation " +
						"is reset.",
					Type:     tools.ParamTypeString,
					Required: true,
				},
				{
					Key: ParamTasksID,
					Description: fmt.Sprintf(
						"A short, unique identifier for this task. Can be used to link sub-tasks with %q",
						ParamTasksParentID),
					Type:     tools.ParamTypeString,
					Required: true,
				},
				{
					Key:         ParamTasksParentID,
					Description: "If this is a sub-task of another task, provide the unique ID of its parent",
					Type:        tools.ParamTypeString,
					Required:    false,
				},
				{
					Key: ParamTasksDependencies,
					Description: "Mark other tasks or subtasks as dependencies, meaning that they must be completed " +
						"before this task, by including their IDs.",
					Type:     tools.ParamTypeArray,
					ItemType: tools.ParamTypeString,
					Required: false,
				},
			},
		},
		{
			Key:         ParamContext,
			Description: "An array of 1 or more items of context",
			Type:        tools.ParamTypeArray,
			Required:    false,
			ItemType:    tools.ParamTypeObject,
			NestedParams: tools.Params{
				{
					Key: ParamContextContent,
					Description: "A piece of context you want to associate with one or more tasks in order to help " +
						"you stay on track and implement the user's request.",
					Type:     tools.ParamTypeString,
					Required: true,
				},
				{
					Key: ParamContextType,
					Description: fmt.Sprintf(
						"The type of context this represents. %q is a snippet or long section of source code from "+
							"e.g. the %q tool. %q is a decision made about the design of the solution that will be "+
							"developed as part of `work_process`. %q is reference material about a 3rd party library"+
							"or external service. %q is a constraint imposed either by the user or the underlying "+
							"codebase. %q is something that is commonly used by other similar parts of the code.",
						plan.ContextTypeCode, tools.ToolReadFile, plan.ContextTypeDecision, plan.ContextTypeReference,
						plan.ContextTypeConstraint, plan.ContextTypeConvention),
					Type: tools.ParamTypeString,
					EnumStringValues: tools.ToStrings([]plan.ContextType{
						plan.ContextTypeCode, plan.ContextTypeConstraint, plan.ContextTypeDecision,
						plan.ContextTypeReference,
					}),
					Required: true,
				},
				{
					Key:         ParamContextID,
					Description: "A short, unique identifier for this piece of context.",
					Type:        tools.ParamTypeString,
					Required:    true,
				},
				{
					Key:         ParamContextOwners,
					Description: "The IDs of the tasks for which this piece of context is relevant",
					Type:        tools.ParamTypeArray,
					ItemType:    tools.ParamTypeString,
					Required:    true,
				},
			},
		},
	}
}

func (p *PlanAdd) Run(_ context.Context, args tools.Args) (string, error) {
	var tasksAdded, contextAdded int

	if tasks := args.GetArgsObjectSlice(ParamTasks); tasks != nil {
		if err := p.handleTasks(tasks); err != nil {
			return "", err
		}

		tasksAdded = len(tasks)
	}

	if context := args.GetArgsObjectSlice(ParamContext); context != nil {
		if err := p.handleContext(context); err != nil {
			return "", err
		}

		contextAdded = len(context)
	}

	addedAny := tasksAdded > 0 || contextAdded > 0
	if !addedAny {
		return "", fmt.Errorf("no valid tasks or context items were found in this tool call")
	}

	return fmt.Sprintf("Added %d tasks and %d context items", tasksAdded, contextAdded), nil
}

func (p *PlanAdd) handleTasks(tasks []tools.Args) error {
	for taskIdx, rawTask := range tasks {
		slog.Debug("Task details", "num", taskIdx, "task", rawTask)
		id := rawTask.GetString(ParamTasksID)
		content := rawTask.GetString(ParamTasksContent)
		parentID := rawTask.GetString(ParamTasksParentID)
		dependencies := rawTask.GetStringSlice(ParamTasksDependencies)

		task := plan.NewTaskItem(*id, *content, plan.OperationAdd)

		if parentID != nil {
			task = task.SetParent(*parentID)
		}

		if dependencies != nil {
			task = task.SetDependencies(dependencies...)
		}

		item := &plan.ItemUnion{TaskItem: task}
		if err := p.PlanManager.HandleItem(item); err != nil {
			return fmt.Errorf("failed to add task with index %d: %w", taskIdx, err)
		}
	}

	return nil
}

func (p *PlanAdd) handleContext(context []tools.Args) error {
	for contextIdx, rawContext := range context {
		slog.Debug("Context details", "num", contextIdx, "context", rawContext)
		rawContextType := rawContext.GetString(ParamContextType)
		content := rawContext.GetString(ParamContextContent)
		id := rawContext.GetString(ParamContextID)
		owners := rawContext.GetStringSlice(ParamContextOwners)

		contextItem := plan.NewContextItem(plan.ContextType(*rawContextType), *content, plan.OperationAdd).
			SetOwners(owners...).
			SetID(*id)

		item := &plan.ItemUnion{ContextItem: contextItem}
		if err := p.PlanManager.HandleItem(item); err != nil {
			return fmt.Errorf("failed to add context with index %d: %w", contextIdx, err)
		}
	}

	return nil
}
