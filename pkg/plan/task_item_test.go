package plan_test

import (
	"testing"
	"time"

	"github.com/cneill/smoke/pkg/plan"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTaskItem_OK(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		input         *plan.TaskItem
		errorContains string
	}{
		{
			name:          "nil",
			input:         nil,
			errorContains: "task item is empty",
		},
		{
			name:          "empty",
			input:         &plan.TaskItem{},
			errorContains: "missing base item properties",
		},
		{
			name: "mismatched item type",
			input: &plan.TaskItem{
				BaseItem: &plan.BaseItem{
					ID: "task1", Time: time.Now(), ItemType: plan.ItemTypeCompletion, Operation: plan.OperationAdd,
				},
			},
			errorContains: "mismatched item type",
		},
		{
			name:          "missing content",
			input:         &plan.TaskItem{BaseItem: plan.NewBaseItem(plan.ItemTypeTask, plan.OperationAdd)},
			errorContains: "missing content",
		},
		{
			name:          "valid",
			input:         plan.NewTaskItem("task1", "task content", plan.OperationAdd),
			errorContains: "",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			err := test.input.OK()

			if test.errorContains == "" {
				require.NoError(t, err)
			} else {
				require.ErrorContains(t, err, test.errorContains)
			}
		})
	}
}

func TestTaskItemCreation(t *testing.T) {
	t.Parallel()

	content := "Implement authentication"
	task := plan.NewTaskItem("task1", content, plan.OperationAdd)

	assert.Equal(t, content, task.Content)
	assert.NotEmpty(t, task.ID)
	assert.NotZero(t, task.Time)
	assert.Empty(t, task.Dependencies)
	assert.Equal(t, plan.ItemTypeTask, task.ItemType)
}

func TestTaskItemBuilders(t *testing.T) {
	t.Parallel()

	task := plan.NewTaskItem("task1", "test task", plan.OperationAdd).
		SetParent("parent-id").
		SetDependencies("dep1", "dep2")

	assert.Equal(t, "parent-id", task.Parent)
	assert.Len(t, task.Dependencies, 2)
	assert.Equal(t, "dep1", task.Dependencies[0])
	assert.Equal(t, "dep2", task.Dependencies[1])
}
