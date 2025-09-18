package plan_test

import (
	"testing"
	"time"

	"github.com/cneill/smoke/pkg/plan"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestContextItem_OK(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		input         *plan.ContextItem
		errorContains string
	}{
		{
			name:          "nil",
			input:         nil,
			errorContains: "context item is empty",
		},
		{
			name:          "empty",
			input:         &plan.ContextItem{},
			errorContains: "missing base item properties",
		},
		{
			name: "mismatched item type",
			input: &plan.ContextItem{
				BaseItem: &plan.BaseItem{
					ID: "task1", Time: time.Now(), ItemType: plan.ItemTypeTask, Operation: plan.OperationAdd,
				},
			},
			errorContains: "mismatched item type",
		},
		{
			name:          "unknown context type",
			input:         plan.NewContextItem("unknown", "context", plan.OperationAdd).SetOwners("task1"),
			errorContains: "unknown context type",
		},
		{
			name:          "missing content",
			input:         plan.NewContextItem(plan.ContextTypeCode, "", plan.OperationAdd).SetOwners("task1"),
			errorContains: "missing content",
		},
		{
			name:          "valid",
			input:         plan.NewContextItem(plan.ContextTypeCode, "context", plan.OperationAdd).SetOwners("task1"),
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

func TestContextItemCreation(t *testing.T) {
	t.Parallel()

	content := "API documentation"
	ctx := plan.NewContextItem(plan.ContextTypeReference, content, plan.OperationAdd)

	assert.Equal(t, content, ctx.Content)
	assert.Equal(t, plan.ContextTypeReference, ctx.ContextType)
	assert.Empty(t, ctx.Owners)
}

func TestContextItemSetOwners(t *testing.T) {
	t.Parallel()

	ctx := plan.NewContextItem(plan.ContextTypeCode, "code snippet", plan.OperationAdd).
		SetOwners("task1", "task2")

	assert.Len(t, ctx.Owners, 2)
	assert.Equal(t, "task1", ctx.Owners[0])
	assert.Equal(t, "task2", ctx.Owners[1])
}
