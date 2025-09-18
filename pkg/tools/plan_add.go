package tools

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/cneill/smoke/pkg/plan"
)

const (
	PlanAddTasks             = "tasks"
	PlanAddTasksContent      = "content"
	PlanAddTasksID           = "id"
	PlanAddTasksParentID     = "parent_id"
	PlanAddTasksDependencies = "dependencies"

	PlanAddContext        = "context"
	PlanAddContextType    = "type"
	PlanAddContextContent = "content"
	PlanAddContextID      = "id"
	PlanAddContextOwners  = "owners"
)

type PlanAddTool struct {
	ProjectPath string
	SessionName string
	PlanManager *plan.Manager
}

func NewPlanAddTool(projectPath, sessionName string) Tool {
	return &PlanAddTool{
		ProjectPath: projectPath,
		SessionName: sessionName,
	}
}

func (p *PlanAddTool) Name() string { return ToolPlanAdd }
func (p *PlanAddTool) Description() string {
	examples := CollectExamples(p.Examples()...)

	return "Add tasks and subtasks to the plan during `plan_process` that will be executed during `work_process`, " +
		"or pieces of context linked to those tasks that are relevant to completing them. If your plan will require " +
		"editing multiple functions/methods extensively, create subtasks for each. If you are implementing simple " +
		"getter/setter methods, you can consolidate to a single, clear task description to cover them all." + examples
}

func (p *PlanAddTool) SetPlanManager(manager *plan.Manager) { p.PlanManager = manager }

func (p *PlanAddTool) Examples() Examples { //nolint:funlen
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
		{
			Description: "Add some context about the existing codebase to make it clearer what work needs to be done " +
				"to complete the task.",
			Args: Args{
				PlanAddTasks: []Args{
					{
						PlanAddTasksID: "vendor_error_handling",
						PlanAddTasksContent: "Improve the error-handling for the VendorClient struct in the vendor " +
							"package by using named errors to make it more testable and consistent.",
					},
				},
				PlanAddContext: []Args{
					{
						PlanAddContextID: "vendor_defined_errors",
						PlanAddContextContent: "The vendor package contains the ErrTimeout, ErrConnectionRefused, " +
							"ErrUnauthorized, and ErrUnknown error types.",
						PlanAddContextOwners: "vendor_error_handling",
						PlanAddContextType:   plan.ContextTypeCode,
					},
					{
						PlanAddContextID: "vendor_undefined_errors",
						PlanAddContextContent: "VendorClient methods return several identical error strings in " +
							"multiple locations that do not have corresponding types.",
						PlanAddContextOwners: "vendor_error_handling",
						PlanAddContextType:   plan.ContextTypeCode,
					},
					{
						PlanAddContextID: "vendor_api_details",
						PlanAddContextContent: "The package comment at the top of `pkg/vendor/vendor.go` includes a " +
							"list of possible errors returned by the vendor REST API, as defined in their docs.",
						PlanAddContextOwners: "vendor_error_handling",
						PlanAddContextType:   plan.ContextTypeReference,
					},
					{
						PlanAddContextID: "implementation_constraints",
						PlanAddContextContent: "Do not create named errors for every rare, one-off error. Focus on " +
							"naming errors that can occur in multiple different places.",
						PlanAddContextOwners: "vendor_error_handling",
						PlanAddContextType:   plan.ContextTypeConstraint,
					},
				},
			},
		},
		{
			Description: "Add tasks with dependencies to ensure proper execution order",
			Args: Args{
				PlanAddTasks: []Args{
					{
						PlanAddTasksID:      "database_migration",
						PlanAddTasksContent: "Create database migration scripts for the new user table schema",
					},
					{
						PlanAddTasksID:           "user_model",
						PlanAddTasksContent:      "Implement the User model struct with validation methods",
						PlanAddTasksDependencies: []string{"database_migration"},
					},
					{
						PlanAddTasksID:           "user_repository",
						PlanAddTasksContent:      "Create UserRepository with CRUD operations",
						PlanAddTasksDependencies: []string{"user_model", "database_migration"},
					},
					{
						PlanAddTasksID:           "user_service",
						PlanAddTasksContent:      "Implement UserService business logic layer",
						PlanAddTasksDependencies: []string{"user_repository"},
					},
				},
			},
		},
		{
			Description: "Add a complex multi-level task hierarchy for refactoring a large module",
			Args: Args{
				PlanAddTasks: []Args{
					{
						PlanAddTasksID:      "refactor_auth_module",
						PlanAddTasksContent: "Refactor the entire authentication module to use JWT tokens instead of sessions",
					},
					{
						PlanAddTasksID:       "refactor_auth_backend",
						PlanAddTasksContent:  "Update backend authentication logic",
						PlanAddTasksParentID: "refactor_auth_module",
					},
					{
						PlanAddTasksID:       "implement_jwt_generation",
						PlanAddTasksContent:  "Create JWT token generation and signing logic",
						PlanAddTasksParentID: "refactor_auth_backend",
					},
					{
						PlanAddTasksID:       "implement_jwt_validation",
						PlanAddTasksContent:  "Create JWT token validation and parsing logic",
						PlanAddTasksParentID: "refactor_auth_backend",
					},
					{
						PlanAddTasksID:           "update_auth_middleware",
						PlanAddTasksContent:      "Update authentication middleware to use JWT tokens",
						PlanAddTasksParentID:     "refactor_auth_backend",
						PlanAddTasksDependencies: []string{"implement_jwt_generation", "implement_jwt_validation"},
					},
					{
						PlanAddTasksID:       "refactor_auth_frontend",
						PlanAddTasksContent:  "Update frontend authentication flow",
						PlanAddTasksParentID: "refactor_auth_module",
					},
					{
						PlanAddTasksID:       "update_login_component",
						PlanAddTasksContent:  "Modify login component to handle JWT tokens",
						PlanAddTasksParentID: "refactor_auth_frontend",
					},
					{
						PlanAddTasksID:       "update_auth_service",
						PlanAddTasksContent:  "Update frontend auth service to store and send JWT tokens",
						PlanAddTasksParentID: "refactor_auth_frontend",
					},
				},
			},
		},
		{
			Description: "Add a task with all four types of context to demonstrate comprehensive planning",
			Args: Args{
				PlanAddTasks: []Args{
					{
						PlanAddTasksID:      "implement_rate_limiting",
						PlanAddTasksContent: "Add rate limiting to the API endpoints to prevent abuse",
					},
				},
				PlanAddContext: []Args{
					{
						PlanAddContextID: "existing_middleware_code",
						PlanAddContextContent: "The current middleware stack is defined in pkg/middleware/chain.go " +
							"and uses the standard net/http Handler interface",
						PlanAddContextOwners: []string{"implement_rate_limiting"},
						PlanAddContextType:   plan.ContextTypeCode,
					},
					{
						PlanAddContextID: "rate_limit_algorithm_decision",
						PlanAddContextContent: "Use a sliding window algorithm with Redis backend for distributed " +
							"rate limiting across multiple server instances",
						PlanAddContextOwners: []string{"implement_rate_limiting"},
						PlanAddContextType:   plan.ContextTypeDecision,
					},
					{
						PlanAddContextID: "redis_rate_limiter_reference",
						PlanAddContextContent: "The go-redis/redis_rate library (github.com/go-redis/redis_rate/v10) " +
							"provides a proven implementation of sliding window rate limiting with Redis",
						PlanAddContextOwners: []string{"implement_rate_limiting"},
						PlanAddContextType:   plan.ContextTypeReference,
					},
					{
						PlanAddContextID: "rate_limit_constraints",
						PlanAddContextContent: "Rate limits must be configurable per endpoint, with default of 100 " +
							"requests per minute for authenticated users and 20 for anonymous users",
						PlanAddContextOwners: []string{"implement_rate_limiting"},
						PlanAddContextType:   plan.ContextTypeConstraint,
					},
				},
			},
		},
	}
}

func (p *PlanAddTool) Params() Params { //nolint:funlen
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
				{
					Key: PlanAddTasksDependencies,
					Description: "Mark other tasks or subtasks as dependencies, meaning that they must be completed " +
						"before this task, by including their IDs.",
					Type:     ParamTypeArray,
					ItemType: ParamTypeString,
					Required: false,
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
							"developed as part of `work_process`. %q is reference material about a 3rd party library"+
							"or external service. %q is a constraint imposed either by the user or the underlying "+
							"codebase. %q is something that is commonly used by other similar parts of the code.",
						plan.ContextTypeCode, ToolReadFile, plan.ContextTypeDecision, plan.ContextTypeReference,
						plan.ContextTypeConstraint, plan.ContextTypeConvention),
					Type: ParamTypeString,
					EnumStringValues: ToStrings([]plan.ContextType{
						plan.ContextTypeCode, plan.ContextTypeConstraint, plan.ContextTypeDecision,
						plan.ContextTypeReference,
					}),
					Required: true,
				},
				{
					Key:         PlanAddContextID,
					Description: "A short, unique identifier for this piece of context.",
					Type:        ParamTypeString,
					Required:    true,
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
	performedAdd := false

	if tasks := args.GetArgsObjectSlice(PlanAddTasks); tasks != nil {
		if err := p.handleTasks(tasks); err != nil {
			return "", err
		}

		performedAdd = true
	}

	if context := args.GetArgsObjectSlice(PlanAddContext); context != nil {
		if err := p.handleContext(context); err != nil {
			return "", err
		}

		performedAdd = true
	}

	if !performedAdd {
		return "", fmt.Errorf("no valid tasks or context items were found in this tool call")
	}

	return "", nil
}

func (p *PlanAddTool) handleTasks(tasks []Args) error {
	for taskIdx, rawTask := range tasks {
		slog.Debug("Task details", "num", taskIdx, "task", rawTask)
		id := rawTask.GetString(PlanAddTasksID)
		content := rawTask.GetString(PlanAddTasksContent)
		parentID := rawTask.GetString(PlanAddTasksParentID)
		dependencies := rawTask.GetStringSlice(PlanAddTasksDependencies)

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

func (p *PlanAddTool) handleContext(context []Args) error {
	for contextIdx, rawContext := range context {
		slog.Debug("Context details", "num", contextIdx, "context", rawContext)
		rawContextType := rawContext.GetString(PlanAddContextType)
		content := rawContext.GetString(PlanAddContextContent)
		id := rawContext.GetString(PlanAddContextID)
		owners := rawContext.GetStringSlice(PlanAddContextOwners)

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
