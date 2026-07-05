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

	fmt.Fprint(builder, "\n# Pending plan tasks\n\n")

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

	fmt.Fprint(builder, "\n# Completed plan tasks\n\n")

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
