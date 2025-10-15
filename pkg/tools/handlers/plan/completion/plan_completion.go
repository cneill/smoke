package plancompletion

import (
	"context"
	"fmt"
	"strings"

	"github.com/cneill/smoke/pkg/plan"
	"github.com/cneill/smoke/pkg/tools"
)

const (
	Name = "plan_completion"

	ParamTaskIDs = "task_ids"
	ParamContent = "content"
	ParamStatus  = "status"
)

type PlanCompletion struct {
	ProjectPath string
	SessionName string
	PlanManager *plan.Manager
}

func New(projectPath, sessionName string) tools.Tool {
	return &PlanCompletion{
		ProjectPath: projectPath,
		SessionName: sessionName,
	}
}

func (p *PlanCompletion) Name() string { return Name }
func (p *PlanCompletion) Description() string {
	examples := tools.CollectExamples(p.Examples()...)

	return fmt.Sprintf("Mark tasks as completed by their IDs, with a note explaining the status of these tasks. If "+
		"you need to completely redefine or remove a task or set of tasks based on user feedback, you can mark them "+
		"as %q to make it clear that these tasks should be ignored during `work_process`.%s",
		plan.CompletionStatusObsolete, examples,
	)
}

func (p *PlanCompletion) SetPlanManager(manager *plan.Manager) { p.PlanManager = manager }

func (p *PlanCompletion) Examples() tools.Examples {
	return tools.Examples{
		{
			Description: "Mark a single task as completed with default success status.",
			Args: tools.Args{
				ParamTaskIDs: []string{"task_id_1"},
				ParamContent: "The Foo() function now returns an error, all call sites are updated to " +
					"account for this, and the relevant tests pass.",
			},
		},
		{
			Description: "Mark multiple tasks as completed with failed status, explaining that the user has failed " +
				"to provide all the necessary context to complete the task.",
			Args: tools.Args{
				ParamTaskIDs: []string{"task_id_1", "task_id_2"},
				ParamContent: "The tests are failing after making modifications to the Foo() function, but " +
					"this appears to be due to a missing API key that I don't have access to.",
				ParamStatus: string(plan.CompletionStatusFailed),
			},
		},
		{
			Description: fmt.Sprintf("Mark a task and its child subtasks as obsolete after user feedback on the "+
				"plan. This is essentially equivalent to deleting the task and will ensure that it does not show up "+
				"in the output of %q.",
				tools.ToolPlanRead,
			),
			Args: tools.Args{
				ParamTaskIDs: []string{"task_id_1"},
				ParamContent: "The user explained that this task is outside the scope of the current " +
					"request and should not be implemented yet.",
				ParamStatus: string(plan.CompletionStatusObsolete),
			},
		},
		{
			Description: "Mark a task as partially completed when some work is done but blocked by external factors",
			Args: tools.Args{
				ParamTaskIDs: []string{"integrate_payment_api"},
				ParamContent: "Implemented the payment gateway client and data models, but cannot complete " +
					"the integration testing because the sandbox environment credentials have not been provided. " +
					"The code is ready but untested against the actual API.",
				ParamStatus: string(plan.CompletionStatusPartial),
			},
		},
		{
			Description: "Complete a parent task and all its subtasks after successfully implementing a feature",
			Args: tools.Args{
				ParamTaskIDs: []string{
					"add_user_authentication", "implement_login", "implement_logout", "add_session_management",
				},
				ParamContent: "Successfully implemented the complete authentication system. Login and logout " +
					"endpoints are working correctly with session management. All unit tests pass and integration " +
					"tests confirm proper authentication flow.",
				ParamStatus: string(plan.CompletionStatusSuccess),
			},
		},
	}
}

func (p *PlanCompletion) Params() tools.Params {
	return tools.Params{
		{
			Key:         ParamTaskIDs,
			Description: "The IDs of the tasks to mark as completed",
			Type:        tools.ParamTypeArray,
			ItemType:    tools.ParamTypeString,
			Required:    true,
		},
		{
			Key: ParamContent,
			Description: "A brief explanation of why the task is being marked completed, and why this particular " +
				"completion status was chosen.",
			Type:     tools.ParamTypeString,
			Required: true,
		},
		{
			Key: ParamStatus,
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
			Type:     tools.ParamTypeString,
			Required: false,
			EnumStringValues: tools.ToStrings([]plan.CompletionStatus{
				plan.CompletionStatusSuccess, plan.CompletionStatusFailed, plan.CompletionStatusPartial,
				plan.CompletionStatusObsolete,
			}),
		},
	}
}

func (p *PlanCompletion) Run(_ context.Context, args tools.Args) (string, error) {
	if p.PlanManager == nil {
		return "", fmt.Errorf("plan manager not set")
	}

	taskIDs := args.GetStringSlice(ParamTaskIDs)
	if taskIDs == nil {
		return "", fmt.Errorf("%w: no task IDs provided", tools.ErrArguments)
	}

	content := args.GetString(ParamContent)

	status := plan.CompletionStatusSuccess

	if statusStr := args.GetString(ParamStatus); statusStr != nil {
		status = plan.CompletionStatus(*statusStr)
	}

	// Validate that all provided IDs correspond to existing tasks
	for _, taskID := range taskIDs {
		item := p.PlanManager.GetItemByID(taskID)
		if item == nil || item.Type() != plan.ItemTypeTask {
			return "", fmt.Errorf("%w: no task found with ID %q", tools.ErrArguments, taskID)
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
