package plan_test

import (
	"encoding/json"
	"testing"

	"github.com/cneill/smoke/pkg/plan"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBaseItem_OK(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		baseItem      *plan.BaseItem
		errorContains string
	}{
		{
			name:          "nil",
			baseItem:      nil,
			errorContains: "missing base item properties",
		},
		{
			name:          "missing ID",
			baseItem:      &plan.BaseItem{},
			errorContains: "missing ID",
		},
		{
			name:          "unknown item type",
			baseItem:      &plan.BaseItem{ID: "id1"},
			errorContains: "unknown item type",
		},
		{
			name:          "unknown operation",
			baseItem:      &plan.BaseItem{ID: "id1", ItemType: plan.ItemTypeTask},
			errorContains: "unknown item operation",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			err := test.baseItem.OK()

			if test.errorContains == "" {
				require.NoError(t, err)
			} else {
				require.ErrorContains(t, err, test.errorContains)
			}
		})
	}
}

// Unit Tests for ItemUnion

func TestItemUnionValidStates(t *testing.T) { //nolint:funlen
	t.Parallel()

	tests := []struct {
		name         string
		setupFunc    func() *plan.ItemUnion
		expectedType plan.ItemType
		shouldError  bool
	}{
		{
			name: "nil",
			setupFunc: func() *plan.ItemUnion {
				return nil
			},
			expectedType: plan.ItemTypeUnknown,
			shouldError:  true,
		},
		{
			name: "invalid - multiple items set (task, context)",
			setupFunc: func() *plan.ItemUnion {
				return &plan.ItemUnion{
					TaskItem:    plan.NewTaskItem("task1", "test", plan.OperationAdd),
					ContextItem: plan.NewContextItem(plan.ContextTypeCode, "test", plan.OperationAdd),
				}
			},
			expectedType: plan.ItemTypeUnknown,
			shouldError:  true,
		},
		{
			name: "invalid - multiple items set (task, completion)",
			setupFunc: func() *plan.ItemUnion {
				return &plan.ItemUnion{
					TaskItem:       plan.NewTaskItem("task1", "test", plan.OperationAdd),
					CompletionItem: plan.NewCompletionItem("completion", "test"),
				}
			},
			expectedType: plan.ItemTypeUnknown,
			shouldError:  true,
		},
		{
			name: "invalid - unknown operation",
			setupFunc: func() *plan.ItemUnion {
				return plan.NewTaskItem("task1", "test", plan.Operation("unknown")).ToUnion()
			},
			expectedType: plan.ItemTypeTask,
			shouldError:  true,
		},
		{
			name: "valid task item (add)",
			setupFunc: func() *plan.ItemUnion {
				return plan.NewTaskItem("task1", "test task", plan.OperationAdd).ToUnion()
			},
			expectedType: plan.ItemTypeTask,
			shouldError:  false,
		},
		{
			name: "valid context item (add)",
			setupFunc: func() *plan.ItemUnion {
				return plan.NewContextItem(plan.ContextTypeCode, "test code", plan.OperationAdd).
					SetOwners("task1").
					ToUnion()
			},
			expectedType: plan.ItemTypeContext,
			shouldError:  false,
		},
		{
			name: "valid completion item (add)",
			setupFunc: func() *plan.ItemUnion {
				return plan.NewCompletionItem("Completed task1", "task1").ToUnion()
			},
			expectedType: plan.ItemTypeCompletion,
			shouldError:  false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			item := test.setupFunc()

			if test.shouldError {
				require.Error(t, item.OK())
			} else {
				require.NoError(t, item.OK())
			}

			assert.Equal(t, test.expectedType, item.Type(), "incorrect item type")
		})
	}
}

func TestItemUnionJSONMarshaling(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		setup    func() *plan.ItemUnion
		itemType plan.ItemType
	}{
		{
			name: "task item",
			setup: func() *plan.ItemUnion {
				return plan.NewTaskItem("task1", "test task", plan.OperationAdd).ToUnion()
			},
			itemType: plan.ItemTypeTask,
		},
		{
			name: "context item",
			setup: func() *plan.ItemUnion {
				return plan.NewContextItem(plan.ContextTypeDecision, "test decision", plan.OperationAdd).
					SetOwners("task1").
					ToUnion()
			},
			itemType: plan.ItemTypeContext,
		},
		{
			name: "completion item",
			setup: func() *plan.ItemUnion {
				return plan.NewCompletionItem("Completed task 1 and 2", "task1", "task2").ToUnion()
			},
			itemType: plan.ItemTypeCompletion,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			original := test.setup()
			assert.Equal(t, test.itemType, original.Type())

			data, err := json.Marshal(original)
			require.NoError(t, err)
			assert.NotEmpty(t, data)

			var restored plan.ItemUnion
			require.NoError(t, json.Unmarshal(data, &restored))

			assert.Equal(t, original.Type(), restored.Type(), "type mismatch between original and restored")
			assert.Equal(t, original.Operation(), restored.Operation(), "operation mismatch between original and restored")
			require.NoError(t, restored.OK())
		})
	}
}

func TestItemUnionCustomUnmarshalJSON(t *testing.T) {
	t.Parallel()

	t.Run("unmarshal task", func(t *testing.T) {
		t.Parallel()

		jsonData := `{"item_type":"task","id":"abc123","time":"2023-01-01T00:00:00Z","content":"test task","parent":"","dependencies":[],"operation":"add"}`

		var item plan.ItemUnion
		require.NoError(t, json.Unmarshal([]byte(jsonData), &item))

		assert.NotNil(t, item.TaskItem)
		assert.Equal(t, "test task", item.TaskItem.Content)
		assert.Equal(t, plan.ItemTypeTask, item.Type())
		assert.NoError(t, item.OK())
	})

	invalidTests := []struct {
		name        string
		jsonContent string
		errorIs     []error
	}{
		{
			name:        "empty",
			jsonContent: `{}`,
			errorIs:     []error{plan.ErrUnknownItemType},
		},
		{
			name:        "invalid_type",
			jsonContent: `{"item_type":"unknown"}`,
			errorIs:     []error{plan.ErrUnknownItemType},
		},
		{
			name:        "empty_type",
			jsonContent: `{"item_type":""}`,
			errorIs:     []error{plan.ErrUnknownItemType},
		},
		{
			name:        "mismatched_type",
			jsonContent: `{"item_type":"task","id":"abc123","time":"2023-01-01T00:00:00Z","operation":"add","owners":["task1"]}`,
			errorIs:     []error{plan.ErrInvalidTaskItem},
		},
	}

	for _, test := range invalidTests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			var item plan.ItemUnion

			err := json.Unmarshal([]byte(test.jsonContent), &item)
			require.Error(t, err)

			for _, isError := range test.errorIs {
				require.ErrorIs(t, err, isError, "unexpected error")
			}
		})
	}
}
