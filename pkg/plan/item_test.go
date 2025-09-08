package plan_test

import (
	"encoding/json"
	"testing"

	"github.com/cneill/smoke/pkg/plan"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Unit Tests for ItemUnion

func TestItemUnionValidStates(t *testing.T) {
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
			name: "valid task item",
			setupFunc: func() *plan.ItemUnion {
				return &plan.ItemUnion{TaskItem: plan.NewTaskItem("test task")}
			},
			expectedType: plan.ItemTypeTask,
			shouldError:  false,
		},
		{
			name: "valid context item",
			setupFunc: func() *plan.ItemUnion {
				return &plan.ItemUnion{ContextItem: plan.NewContextItem(plan.ContextTypeCode, "test code")}
			},
			expectedType: plan.ItemTypeContext,
			shouldError:  false,
		},
		{
			name: "valid completion item",
			setupFunc: func() *plan.ItemUnion {
				return &plan.ItemUnion{CompletionItem: plan.NewCompletionItem("task1")}
			},
			expectedType: plan.ItemTypeCompletion,
			shouldError:  false,
		},
		{
			name: "invalid - multiple items set",
			setupFunc: func() *plan.ItemUnion {
				return &plan.ItemUnion{
					TaskItem:    plan.NewTaskItem("test"),
					ContextItem: plan.NewContextItem(plan.ContextTypeCode, "test"),
				}
			},
			expectedType: plan.ItemTypeUnknown,
			shouldError:  true,
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
				return &plan.ItemUnion{TaskItem: plan.NewTaskItem("test task")}
			},
			itemType: plan.ItemTypeTask,
		},
		{
			name: "context item",
			setup: func() *plan.ItemUnion {
				return &plan.ItemUnion{ContextItem: plan.NewContextItem(plan.ContextTypeDecision, "test decision")}
			},
			itemType: plan.ItemTypeContext,
		},
		{
			name: "completion item",
			setup: func() *plan.ItemUnion {
				return &plan.ItemUnion{CompletionItem: plan.NewCompletionItem("task1", "task2")}
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
			assert.NotEmpty(t, data)
			require.NoError(t, err)

			var restored plan.ItemUnion
			require.NoError(t, json.Unmarshal(data, &restored))

			assert.Equal(t, original.Type(), restored.Type(), "type mismatch between original and restored")
			require.NoError(t, restored.OK())
		})
	}
}

func TestItemUnionCustomUnmarshalJSON(t *testing.T) {
	t.Parallel()

	t.Run("unmarshal task", func(t *testing.T) {
		t.Parallel()

		jsonData := `{"item_type":"task","id":"abc123","time":"2023-01-01T00:00:00Z","content":"test task","parent":"","dependencies":[]}`

		var item plan.ItemUnion
		require.NoError(t, json.Unmarshal([]byte(jsonData), &item))

		assert.NotNil(t, item.TaskItem)
		assert.Equal(t, "test task", item.TaskItem.Content)
		assert.Equal(t, plan.ItemTypeTask, item.Type())
	})

	t.Run("unmarshal unknown type", func(t *testing.T) {
		t.Parallel()

		jsonData := `{"item_type":"unknown","id":"abc123"}`

		var item plan.ItemUnion

		err := json.Unmarshal([]byte(jsonData), &item)
		require.Error(t, err)
		require.EqualError(t, err, "unknown item type: \"unknown\"", "Unexpected error message: %v", err.Error())
	})
}

// Unit Tests for TaskItem

func TestTaskItemCreation(t *testing.T) {
	t.Parallel()

	content := "Implement authentication"
	task := plan.NewTaskItem(content)

	assert.Equal(t, content, task.Content)
	assert.NotEmpty(t, task.ID)
	assert.NotZero(t, task.Time)
	assert.Empty(t, task.Dependencies)
	assert.Equal(t, plan.ItemTypeTask, task.ItemType)
}

func TestTaskItemBuilders(t *testing.T) {
	t.Parallel()

	task := plan.NewTaskItem("test task").
		SetParent("parent-id").
		SetDependencies("dep1", "dep2")

	assert.Equal(t, "parent-id", task.Parent)
	assert.Len(t, task.Dependencies, 2)
	assert.Equal(t, "dep1", task.Dependencies[0])
	assert.Equal(t, "dep2", task.Dependencies[1])
}

// Unit Tests for ContextItem

func TestContextItemCreation(t *testing.T) {
	t.Parallel()

	content := "API documentation"
	ctx := plan.NewContextItem(plan.ContextTypeReference, content)

	assert.Equal(t, content, ctx.Content)
	assert.Equal(t, plan.ContextTypeReference, ctx.ContextType)
	assert.Empty(t, ctx.Owners)
}

func TestContextItemSetOwners(t *testing.T) {
	t.Parallel()

	ctx := plan.NewContextItem(plan.ContextTypeCode, "code snippet").
		SetOwners("task1", "task2")

	assert.Len(t, ctx.Owners, 2)
	assert.Equal(t, "task1", ctx.Owners[0])
	assert.Equal(t, "task2", ctx.Owners[1])
}

// Unit Tests for CompletionItem

func TestCompletionItemCreation(t *testing.T) {
	t.Parallel()

	comp := plan.NewCompletionItem("task1", "task2")

	assert.Len(t, comp.TaskIDs, 2)
	assert.Equal(t, "task1", comp.TaskIDs[0])
	assert.Equal(t, "task2", comp.TaskIDs[1])
	assert.Equal(t, plan.CompletionStatusSuccess, comp.Status)
}
