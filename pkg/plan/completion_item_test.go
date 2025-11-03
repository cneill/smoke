package plan_test

import (
	"testing"
	"time"

	"github.com/cneill/smoke/pkg/plan"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCompletionItem_OK(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		input         *plan.CompletionItem
		errorContains string
	}{
		{
			name:          "nil",
			input:         nil,
			errorContains: "completion item is empty",
		},
		{
			name:          "empty",
			input:         &plan.CompletionItem{},
			errorContains: "missing base item properties",
		},
		{
			name: "mismatched item type",
			input: &plan.CompletionItem{
				BaseItem: &plan.BaseItem{
					ID: "task1", Time: time.Now(), ItemType: plan.ItemTypeTask, Operation: plan.OperationAdd,
				},
			},
			errorContains: "mismatched item type",
		},
		{
			name:          "unknown completion status",
			input:         plan.NewCompletionItem("complete", "task1").SetStatus("unknown"),
			errorContains: "unknown completion status",
		},
		{
			name:          "missing content",
			input:         plan.NewCompletionItem("", "task1"),
			errorContains: "missing content",
		},
		{
			name:          "missing task IDs",
			input:         plan.NewCompletionItem("completed"),
			errorContains: "missing task IDs",
		},
		{
			name:          "valid",
			input:         plan.NewCompletionItem("completed", "task1"),
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

func TestCompletionItemCreation(t *testing.T) {
	t.Parallel()

	comp := plan.NewCompletionItem("Successfully completed tasks 1 and 2", "task1", "task2")

	assert.Len(t, comp.TaskIDs, 2)
	assert.Equal(t, "task1", comp.TaskIDs[0])
	assert.Equal(t, "task2", comp.TaskIDs[1])
	assert.Equal(t, plan.CompletionStatusSuccess, comp.Status)
}
