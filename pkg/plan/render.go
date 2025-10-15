package plan

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

func (m *Manager) Markdown() string {
	builder := &strings.Builder{}
	m.printPendingTasks(builder)
	m.printCompletedTasks(builder)

	return builder.String()
}

func (m *Manager) printPendingTasks(builder *strings.Builder) {
	pending := m.GetPendingTasks()
	if len(pending) == 0 {
		return
	}

	fmt.Fprintln(builder, "Pending plan tasks:")

	for _, task := range pending {
		if task.Parent != "" {
			continue
		}

		m.printNested(task, 0, false, builder)
	}
}

func (m *Manager) printCompletedTasks(builder *strings.Builder) {
	completedIndexes := m.Completed()
	if len(completedIndexes) == 0 {
		return
	}

	fmt.Fprintln(builder, "Successfully completed plan tasks:")

	ordered := []*TaskItem{}

	for id, status := range completedIndexes {
		if status == CompletionStatusSuccess {
			item := m.GetItemByID(id)
			if item == nil {
				continue
			}

			ordered = append(ordered, item.TaskItem)
		}
	}

	sort.Slice(ordered, func(i, j int) bool {
		return ordered[i].Time.Before(ordered[j].Time)
	})

	for _, task := range ordered {
		m.printNested(task, 0, false, builder)
	}
}

func (m *Manager) printNested(task *TaskItem, indent int, addStatus bool, builder *strings.Builder) {
	var (
		spacing = strings.Repeat("\t", indent)
		status  string
	)

	if addStatus {
		status = "; status=pending"

		// See if we have a completed status to replace "pending"
		var completedStatus string

		for id, taskStatus := range m.Completed() {
			if task.ID == id {
				completedStatus = string(taskStatus)
				break
			}
		}

		if completedStatus != "" {
			status = "; status=" + completedStatus
		}
	}

	fmt.Fprintf(builder, "%s* `%s` (added %s%s): %s\n", spacing, task.ID, task.Time.Format(time.DateTime), status, task.Content)

	if context := m.GetContextFor(task.ID); len(context) > 0 {
		fmt.Fprintf(builder, "%s\t* **Context for `%s`:**\n", spacing, task.ID)

		for _, context := range context {
			fmt.Fprintf(builder, "%s\t\t* `%s` (type=%s): %s\n", spacing, context.ID, context.ContextType, context.Content)
		}
	}

	if children := m.GetChildrenFor(task.ID); len(children) > 0 {
		fmt.Fprintf(builder, "%s\t* **Children for `%s`:**\n", spacing, task.ID)

		for _, child := range children {
			m.printNested(child, indent+2, true, builder)
		}
	}
}

// func (p *Manager) Markdown() string {
// 	allItems := p.AllItems()
// 	completed := p.Completed()
//
// 	// Map to track latest item per ID
// 	// TODO: use 'updated' map from Manager to handle this...?
// 	latestItems := make(map[string]*ItemUnion)
//
// 	for _, item := range allItems {
// 		latestItems[item.ID()] = item
// 	}
//
// 	tasks := make([]*TaskItem, 0, len(latestItems))
// 	contexts := make([]*ContextItem, 0, len(latestItems))
//
// 	for _, item := range latestItems {
// 		switch item.Type() { //nolint:exhaustive
// 		case ItemTypeTask:
// 			tasks = append(tasks, item.TaskItem)
// 		case ItemTypeContext:
// 			contexts = append(contexts, item.ContextItem)
// 			// TODO: Completions
// 		}
// 	}
//
// 	// Build task tree
// 	taskMap := make(map[string]*TaskItem)
// 	for _, task := range tasks {
// 		taskMap[task.ID] = task
// 	}
//
// 	rootTasks := []*TaskItem{}
// 	childTasks := make(map[string][]*TaskItem)
//
// 	for _, task := range tasks {
// 		if task.Parent == "" {
// 			rootTasks = append(rootTasks, task)
// 		} else {
// 			childTasks[task.Parent] = append(childTasks[task.Parent], task)
// 		}
// 	}
//
// 	// Recursive function to format tasks with indentation
// 	var formatTasks func(taskIDs []string, indentLevel int) string
//
// 	formatTasks = func(taskIDs []string, indentLevel int) string {
// 		var sb strings.Builder
//
// 		indent := strings.Repeat("  ", indentLevel)
//
// 		for _, taskID := range taskIDs {
// 			task := taskMap[taskID]
//
// 			status := "pending"
// 			if comp, ok := completed[task.ID]; ok {
// 				status = string(comp)
// 			}
//
// 			sb.WriteString(fmt.Sprintf("%s- %s (id: %s) [%s]\n", indent, task.Content, task.ID, status))
//
// 			if len(task.Dependencies) > 0 {
// 				sb.WriteString(fmt.Sprintf("%s  Dependencies: %s\n", indent, strings.Join(task.Dependencies, ", ")))
// 			}
//
// 			// Contexts for this task
// 			ctxs := p.GetContextFor(task.ID)
// 			if len(ctxs) > 0 {
// 				sb.WriteString(indent + "  Contexts:\n")
//
// 				for _, ctx := range ctxs {
// 					sb.WriteString(fmt.Sprintf("%s    - %s: %s (id: %s)\n", indent, ctx.ContextType, ctx.Content, ctx.ID))
// 				}
// 			}
//
// 			// Children
// 			if children, ok := childTasks[task.ID]; ok {
// 				// Sort children by ID for consistency
// 				sort.Slice(children, func(i, j int) bool {
// 					return children[i].ID < children[j].ID
// 				})
//
// 				childIDs := make([]string, len(children))
// 				for i, c := range children {
// 					childIDs[i] = c.ID
// 				}
//
// 				sb.WriteString(formatTasks(childIDs, indentLevel+1))
// 			}
// 		}
//
// 		return sb.String()
// 	}
//
// 	// Start formatting from root tasks
// 	sort.Slice(rootTasks, func(i, j int) bool {
// 		return rootTasks[i].ID < rootTasks[j].ID
// 	})
//
// 	rootIDs := make([]string, len(rootTasks))
// 	for i, t := range rootTasks {
// 		rootIDs[i] = t.ID
// 	}
//
// 	output := formatTasks(rootIDs, 0)
//
// 	if output == "" {
// 		output = "No tasks in the plan."
// 	}
//
// 	// Append any standalone contexts (not owned by tasks, but unlikely)
// 	var allContextsStr string
//
// 	if len(contexts) > 0 {
// 		allContextsStr = "\nAll Contexts:\n"
//
// 		for _, ctx := range contexts {
// 			var ownersStr string
// 			if len(ctx.Owners) > 0 {
// 				ownersStr = fmt.Sprintf(" (owners: %s)", strings.Join(ctx.Owners, ", "))
// 			}
//
// 			allContextsStr += fmt.Sprintf("- %s: %s (id: %s)%s\n", ctx.ContextType, ctx.Content, ctx.ID, ownersStr)
// 		}
//
// 		output += allContextsStr
// 	}
//
// 	return output
// }
