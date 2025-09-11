package tools

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/cneill/smoke/pkg/plan"
)

type PlanReadTool struct {
	ProjectPath string
	SessionName string
	PlanManager *plan.Manager
}

func NewPlanReadTool(projectPath, sessionName string) Tool {
	return &PlanReadTool{
		ProjectPath: projectPath,
		SessionName: sessionName,
	}
}

func (p *PlanReadTool) Name() string { return ToolPlanRead }
func (p *PlanReadTool) Description() string {
	examples := CollectExamples(p.Examples()...)

	return "Read the current state of the plan, including all tasks, sub-tasks, dependencies, completions, and " +
		"contexts." + examples
}

func (p *PlanReadTool) SetPlanManager(manager *plan.Manager) { p.PlanManager = manager }

func (p *PlanReadTool) Examples() Examples {
	return Examples{
		{
			Description: "Read the full current state of the plan",
			Args:        Args{},
		},
	}
}

func (p *PlanReadTool) Params() Params {
	return Params{}
}

func (p *PlanReadTool) Run(_ context.Context, _ Args) (string, error) {
	if p.PlanManager == nil {
		return "", fmt.Errorf("plan manager not set")
	}

	allItems := p.PlanManager.AllItems()
	completed := p.PlanManager.Completed()

	// Map to track latest item per ID
	// TODO: use 'updated' map from Manager to handle this...?
	latestItems := make(map[string]*plan.ItemUnion)

	for _, item := range allItems {
		latestItems[item.ID()] = item
	}

	// Collect tasks and contexts
	tasks := make([]*plan.TaskItem, 0, len(latestItems))
	contexts := make([]*plan.ContextItem, 0, len(latestItems))

	for _, item := range latestItems {
		switch item.Type() {
		case plan.ItemTypeTask:
			tasks = append(tasks, item.TaskItem)
		case plan.ItemTypeContext:
			contexts = append(contexts, item.ContextItem)
			// TODO: Completions
		}
	}

	// Build task tree
	taskMap := make(map[string]*plan.TaskItem)
	for _, task := range tasks {
		taskMap[task.ID] = task
	}

	// Root tasks (no parent or parent not found, but since parent must be added first, assume consistent)
	rootTasks := []*plan.TaskItem{}
	childTasks := make(map[string][]*plan.TaskItem)

	for _, task := range tasks {
		if task.Parent == "" {
			rootTasks = append(rootTasks, task)
		} else {
			childTasks[task.Parent] = append(childTasks[task.Parent], task)
		}
	}

	// Recursive function to format tasks with indentation
	var formatTasks func(taskIDs []string, indentLevel int) string

	formatTasks = func(taskIDs []string, indentLevel int) string {
		var sb strings.Builder

		indent := strings.Repeat("  ", indentLevel)

		for _, taskID := range taskIDs {
			task := taskMap[taskID]

			status := "pending"
			if comp, ok := completed[task.ID]; ok {
				status = string(comp)
			}

			sb.WriteString(fmt.Sprintf("%s- %s (id: %s) [%s]\n", indent, task.Content, task.ID, status))

			if len(task.Dependencies) > 0 {
				sb.WriteString(fmt.Sprintf("%s  Dependencies: %s\n", indent, strings.Join(task.Dependencies, ", ")))
			}

			// Contexts for this task
			ctxs := p.PlanManager.GetContextFor(task.ID)
			if len(ctxs) > 0 {
				sb.WriteString(indent + "  Contexts:\n")

				for _, ctx := range ctxs {
					sb.WriteString(fmt.Sprintf("%s    - %s: %s (id: %s)\n", indent, ctx.ContextType, ctx.Content, ctx.ID))
				}
			}

			// Children
			if children, ok := childTasks[task.ID]; ok {
				// Sort children by ID for consistency
				sort.Slice(children, func(i, j int) bool {
					return children[i].ID < children[j].ID
				})

				childIDs := make([]string, len(children))
				for i, c := range children {
					childIDs[i] = c.ID
				}

				sb.WriteString(formatTasks(childIDs, indentLevel+1))
			}
		}

		return sb.String()
	}

	// Start formatting from root tasks
	sort.Slice(rootTasks, func(i, j int) bool {
		return rootTasks[i].ID < rootTasks[j].ID
	})

	rootIDs := make([]string, len(rootTasks))
	for i, t := range rootTasks {
		rootIDs[i] = t.ID
	}

	output := formatTasks(rootIDs, 0)

	if output == "" {
		output = "No tasks in the plan."
	}

	// Append any standalone contexts (not owned by tasks, but unlikely)
	var allContextsStr string

	if len(contexts) > 0 {
		allContextsStr = "\nAll Contexts:\n"

		for _, ctx := range contexts {
			var ownersStr string
			if len(ctx.Owners) > 0 {
				ownersStr = fmt.Sprintf(" (owners: %s)", strings.Join(ctx.Owners, ", "))
			}

			allContextsStr += fmt.Sprintf("- %s: %s (id: %s)%s\n", ctx.ContextType, ctx.Content, ctx.ID, ownersStr)
		}

		output += allContextsStr
	}

	return output, nil
}
