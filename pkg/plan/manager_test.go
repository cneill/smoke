package plan_test

import (
	"bytes"
	"sync"
	"testing"
	"time"

	"github.com/cneill/smoke/pkg/plan"
	"github.com/cneill/smoke/pkg/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Helpers
func createTestManager(t *testing.T) (*plan.Manager, *bytes.Buffer) {
	t.Helper()

	buf := &bytes.Buffer{}
	mgr := plan.NewManager(buf)

	return mgr, buf
}

// Integration Tests for Manager

func TestEmptyManager(t *testing.T) {
	t.Parallel()

	mgr, buf := createTestManager(t)

	allItems := mgr.AllItems()
	assert.Empty(t, allItems, "expected no items")

	completed := mgr.Completed()
	assert.Empty(t, completed, "expected empty completed")

	assert.Equal(t, 0, buf.Len(), "expected empty buffer")
	assert.True(t, mgr.IsWriting, "expected manager to be writing")
}

func TestManagerAddItem(t *testing.T) {
	t.Parallel()

	mgr, _ := createTestManager(t)
	task := plan.NewTaskItem("task1", "test task", plan.OperationAdd).ToUnion()
	require.NoError(t, mgr.HandleItem(task), "failed to add item")

	allItems := mgr.AllItems()

	assert.Len(t, allItems, 1, "expected 1 item")
	assert.NotNil(t, allItems[0].TaskItem)
	assert.Equal(t, "test task", allItems[0].TaskItem.Content)
}

func TestManagerAddMultipleItems(t *testing.T) {
	t.Parallel()

	mgr, _ := createTestManager(t)

	items := []*plan.ItemUnion{
		plan.NewTaskItem("task1", "do thing", plan.OperationAdd).ToUnion(),
		plan.NewContextItem(plan.ContextTypeCode, "code snippet", plan.OperationAdd).SetOwners("task1").ToUnion(),
		plan.NewCompletionItem("Completed task1", "task1").ToUnion(),
	}

	for _, item := range items {
		require.NoError(t, mgr.HandleItem(item), "failed to add item")
	}

	allItems := mgr.AllItems()
	assert.Len(t, allItems, 3)
	assert.NotNil(t, allItems[0].TaskItem)
	assert.NotNil(t, allItems[1].ContextItem)
	assert.NotNil(t, allItems[2].CompletionItem)
}

func TestManagerTaskUpdateTracking(t *testing.T) {
	t.Parallel()

	mgr, _ := createTestManager(t)
	taskID := "task1"
	task := plan.NewTaskItem(taskID, "test task", plan.OperationAdd).ToUnion()
	require.NoError(t, mgr.HandleItem(task), "failed to add task")

	assert.Len(t, mgr.AllItems(), 1, "expected 1 item")

	updatedTaskContent := "updated test task"
	updatedTask := plan.NewTaskItem(taskID, updatedTaskContent, plan.OperationUpdate).ToUnion()
	require.NoError(t, mgr.HandleItem(updatedTask), "failed to update task")

	updatedItems := mgr.AllItems()
	assert.Len(t, updatedItems, 2, "expected 2 items after update")
	assert.Equal(t, taskID, updatedItems[0].ID())
	assert.Equal(t, taskID, updatedItems[1].ID())

	pending := mgr.GetPendingTasks()
	assert.Len(t, pending, 1, "expecting only 1 pending task after update")
	assert.Equal(t, updatedTaskContent, pending[0].Content, "got unexpected task content after update")

	item := mgr.GetItemByID(taskID)
	assert.Equal(t, plan.ItemTypeTask, item.Type())
	assert.Equal(t, updatedTaskContent, item.TaskItem.Content)
}

func TestManagerContextUpdateTracking(t *testing.T) {
	t.Parallel()

	mgr, _ := createTestManager(t)
	taskID := "task1234"
	task := plan.NewTaskItem(taskID, "test task", plan.OperationAdd).ToUnion()
	require.NoError(t, mgr.HandleItem(task), "failed to add task")

	assert.Len(t, mgr.AllItems(), 1, "expected 1 item")

	contextID := "context1234"
	context := plan.NewContextItem(plan.ContextTypeConstraint, "Don't use third party libraries", plan.OperationAdd).
		SetOwners(taskID).SetID(contextID).ToUnion()
	require.NoError(t, mgr.HandleItem(context), "failed to add task context")

	updatedContextContent := "Use only approved third party libraries"
	updatedContext := plan.NewContextItem(plan.ContextTypeConstraint, updatedContextContent, plan.OperationUpdate).
		SetOwners(taskID).SetID(contextID).ToUnion()
	require.NoError(t, mgr.HandleItem(updatedContext), "failed to update context")

	updatedItems := mgr.AllItems()
	assert.Len(t, updatedItems, 3, "expected 3 items after update")
	assert.Equal(t, taskID, updatedItems[0].ID())

	pending := mgr.GetPendingTasks()
	assert.Len(t, pending, 1, "expecting only 1 pending task after update")
	assert.Equal(t, "test task", pending[0].Content, "got unexpected task content after update")

	contextFor := mgr.GetContextFor(taskID)
	assert.Len(t, contextFor, 1, "expecting only 1 item of context after update")
	assert.Equal(t, updatedContextContent, contextFor[0].Content, "context was not updated")

	item := mgr.GetItemByID(contextID)
	assert.Equal(t, plan.ItemTypeContext, item.Type())
	assert.Equal(t, updatedContextContent, item.ContextItem.Content)
}

func TestManagerCompletionTracking(t *testing.T) {
	t.Parallel()

	mgr, _ := createTestManager(t)
	task1ID := "task1"
	task1 := plan.NewTaskItem(task1ID, "completable task", plan.OperationAdd).ToUnion()
	require.NoError(t, mgr.AddItem(task1))

	task2ID := "task2"
	task2 := plan.NewTaskItem(task2ID, "impossible task", plan.OperationAdd).ToUnion()
	require.NoError(t, mgr.AddItem(task2))

	completion1 := plan.NewCompletionItem("Completed task", task1ID).ToUnion()
	require.NoError(t, mgr.AddItem(completion1))

	completion2 := plan.NewCompletionItem("Failed to complete task", task2ID).SetStatus(plan.CompletionStatusFailed).ToUnion()
	require.NoError(t, mgr.AddItem(completion2))

	allItems := mgr.AllItems()
	assert.Len(t, allItems, 4)

	completed := mgr.Completed()
	assert.Len(t, completed, 2, "expected 2 tasks to be marked completed")

	if status, exists := completed[task1ID]; !exists || status != plan.CompletionStatusSuccess {
		t.Errorf("Expected %s to be completed with success, got %v, %v", task1ID, status, exists)
	}

	if status, exists := completed[task2ID]; !exists || status != plan.CompletionStatusFailed {
		t.Errorf("Expected %s to be completed with failure, got %v, %v", task2ID, status, exists)
	}

	pending := mgr.GetPendingTasks()
	assert.Len(t, pending, 1, "expected 1 pending (failed) task after %s completion", task1ID)

	// Test duplicate completion - should just log a warning, no error
	completionDupe := plan.NewCompletionItem("Completed task", task1ID).ToUnion()
	require.NoError(t, mgr.AddItem(completionDupe))

	completed = mgr.Completed()

	assert.Equal(t, plan.CompletionStatusSuccess, completed[task1ID])
}

func TestManagerHandleInvalidItems(t *testing.T) { //nolint:funlen
	t.Parallel()

	tests := []struct {
		name          string
		items         []*plan.ItemUnion
		errorContains []string
	}{
		{
			name:          "nil",
			items:         []*plan.ItemUnion{nil},
			errorContains: []string{"unknown or empty item operation"},
		},
		{
			name: "unknown operation",
			items: []*plan.ItemUnion{
				plan.NewTaskItem("task1", "task content", "").ToUnion(),
			},
			errorContains: []string{"unknown or empty item operation"},
		},
		{
			name: "invalid task add",
			items: []*plan.ItemUnion{
				plan.NewTaskItem("", "task content", plan.OperationAdd).ToUnion(),
			},
			errorContains: []string{"invalid task item", "missing ID"},
		},
		{
			name: "invalid task add dependencies",
			items: []*plan.ItemUnion{
				plan.NewTaskItem("task2", "task content", plan.OperationAdd).SetDependencies("task1").ToUnion(),
			},
			errorContains: []string{"unknown dependency IDs"},
		},
		{
			name: "task add ID collision",
			items: []*plan.ItemUnion{
				plan.NewTaskItem("task1", "task 1 content", plan.OperationAdd).ToUnion(),
				plan.NewTaskItem("task1", "task 2 content", plan.OperationAdd).ToUnion(),
			},
			errorContains: []string{"item already exists"},
		},
		{
			name: "invalid task update",
			items: []*plan.ItemUnion{
				plan.NewTaskItem("task1", "task content", plan.OperationAdd).ToUnion(),
				plan.NewTaskItem("task1", "", plan.OperationUpdate).ToUnion(),
			},
			errorContains: []string{"missing content"},
		},
		{
			name: "invalid task update dependencies",
			items: []*plan.ItemUnion{
				plan.NewTaskItem("task1", "task content", plan.OperationAdd).ToUnion(),
				plan.NewTaskItem("task1", "task content", plan.OperationUpdate).SetDependencies("task2").ToUnion(),
			},
			errorContains: []string{"unknown dependency IDs"},
		},
		{
			name: "unknown task to update",
			items: []*plan.ItemUnion{
				plan.NewTaskItem("task1", "task content", plan.OperationAdd).ToUnion(),
				plan.NewTaskItem("task2", "updated content", plan.OperationUpdate).ToUnion(),
			},
			errorContains: []string{"does not exist to update"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			var evalErr error

			mgr, _ := createTestManager(t)
			for _, item := range test.items {
				err := mgr.HandleItem(item)
				if err != nil {
					evalErr = err
				}
			}

			if len(test.errorContains) == 0 {
				require.NoError(t, evalErr)
			} else {
				for _, errContains := range test.errorContains {
					require.ErrorContains(t, evalErr, errContains, "unexpected error")
				}
			}
		})
	}
}

func TestManagerWritingDisabled(t *testing.T) {
	t.Parallel()

	buf := &bytes.Buffer{}
	mgr := plan.NewManager(buf)
	mgr.IsWriting = false

	task := plan.NewTaskItem("task1", "test task", plan.OperationAdd).ToUnion()
	require.NoError(t, mgr.AddItem(task))

	assert.Len(t, mgr.AllItems(), 1, "expected 1 item")
	assert.Equal(t, 0, buf.Len(), "expected empty buffer - no writing")
}

func TestManagerFromReader(t *testing.T) {
	t.Parallel()

	logData := `{"item_type":"task","id":"task1","time":"2023-01-01T00:00:00Z","content":"task one","parent":"","dependencies":[],"operation":"add"}
	{"item_type":"completion","id":"comp1","time":"2023-01-01T00:01:00Z","task_ids":["task1"],"status":"success","operation":"add","content":"done!"}
`
	buf := &bytes.Buffer{}
	buf.WriteString(logData)

	mgr, err := plan.ManagerFromReader(buf)
	require.NoError(t, err)

	assert.Len(t, mgr.AllItems(), 2)

	completed := mgr.Completed()

	assert.Len(t, completed, 1)

	if status, exists := completed["task1"]; !exists || status != plan.CompletionStatusSuccess {
		t.Errorf("Expected task1 to be completed, got %v, %v", status, exists)
	}

	assert.True(t, mgr.IsWriting, "expected IsWriting to be true after loading")
}

// Concurrency Tests

func TestManagerFromReaderMalformedJSON(t *testing.T) {
	t.Parallel()

	logData := `{"item_type":"task","id":"task1","time":"2023-01-01T00:00:00Z","content":"task one","parent":"","dependencies":[],"operation":"add"}
{"item_type":"invalid json here`
	buf := &bytes.Buffer{}
	buf.WriteString(logData)

	_, err := plan.ManagerFromReader(buf)
	require.Error(t, err)

	assert.Contains(t, err.Error(), "failed to parse item number 2")
}

func TestManagerConcurrentAdds(t *testing.T) {
	t.Parallel()

	mgr, _ := createTestManager(t)

	var (
		wg                   sync.WaitGroup
		numGoroutines        = 10
		numItemsPerGoroutine = 10
	)

	for range numGoroutines {
		wg.Go(func() {
			for range numItemsPerGoroutine {
				task := plan.NewTaskItem(utils.RandID(32), "task from goroutine", plan.OperationAdd).ToUnion()
				require.NoError(t, mgr.AddItem(task))
				time.Sleep(1 * time.Millisecond) // Small delay for contention
			}
		})
	}

	wg.Wait()

	expectedItems := numGoroutines * numItemsPerGoroutine
	assert.Len(t, mgr.AllItems(), expectedItems)
}
