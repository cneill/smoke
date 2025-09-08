package plan_test

import (
	"bytes"
	"sync"
	"testing"
	"time"

	"github.com/cneill/smoke/pkg/plan"
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
	task := &plan.ItemUnion{TaskItem: plan.NewTaskItem("test task")}
	require.NoError(t, mgr.AddItem(task), "failed to add item")

	assert.Len(t, mgr.AllItems(), 1, "expected 1 item")
}

func TestManagerAddMultipleItems(t *testing.T) {
	t.Parallel()

	mgr, _ := createTestManager(t)

	items := []*plan.ItemUnion{
		{TaskItem: plan.NewTaskItem("task 1")},
		{ContextItem: plan.NewContextItem(plan.ContextTypeCode, "code snippet").SetOwners("task1")},
		{CompletionItem: plan.NewCompletionItem("task1")},
	}

	for _, item := range items {
		require.NoError(t, mgr.AddItem(item), "failed to add item")
	}

	assert.Len(t, mgr.AllItems(), 3)
}

func TestManagerCompletionTracking(t *testing.T) {
	t.Parallel()

	mgr, _ := createTestManager(t)

	task := &plan.ItemUnion{TaskItem: plan.NewTaskItem("completable task")}
	require.NoError(t, mgr.AddItem(task))

	allItems := mgr.AllItems()
	taskID := allItems[0].TaskItem.ID

	completion := &plan.ItemUnion{CompletionItem: plan.NewCompletionItem(taskID)}
	require.NoError(t, mgr.AddItem(completion))

	completed := mgr.Completed()

	assert.Len(t, completed, 1, "expected 1 task to be marked completed")

	if status, exists := completed[taskID]; !exists || status != plan.CompletionStatusSuccess {
		t.Errorf("Expected task to be completed with success, got %v, %v", status, exists)
	}

	// Test duplicate completion - should just log a warning, no error
	completion2 := &plan.ItemUnion{CompletionItem: plan.NewCompletionItem(taskID)}
	require.NoError(t, mgr.AddItem(completion2))

	completed = mgr.Completed()

	assert.Equal(t, plan.CompletionStatusSuccess, completed[taskID])
}

func TestManagerWritingDisabled(t *testing.T) {
	t.Parallel()

	buf := &bytes.Buffer{}
	mgr := plan.NewManager(buf)
	mgr.IsWriting = false

	task := &plan.ItemUnion{TaskItem: plan.NewTaskItem("test task")}
	require.NoError(t, mgr.AddItem(task))

	assert.Len(t, mgr.AllItems(), 1, "expected 1 item")
	assert.Equal(t, 0, buf.Len(), "expected empty buffer - no writing")
}

func TestManagerFromReader(t *testing.T) {
	t.Parallel()

	logData := `{"item_type":"task","id":"task1","time":"2023-01-01T00:00:00Z","content":"task one","parent":"","dependencies":[]}
{"item_type":"completion","id":"comp1","time":"2023-01-01T00:01:00Z","task_ids":["task1"],"status":"success"}
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

	logData := `{"item_type":"task","id":"task1","time":"2023-01-01T00:00:00Z","content":"task one","parent":"","dependencies":[]}
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
		numItemsPerGoroutine = 5
	)

	for range numGoroutines {
		wg.Go(func() {
			for range numItemsPerGoroutine {
				task := &plan.ItemUnion{TaskItem: plan.NewTaskItem("task from goroutine")}
				require.NoError(t, mgr.AddItem(task))
				time.Sleep(1 * time.Millisecond) // Small delay for contention
			}
		})
	}

	wg.Wait()

	expectedItems := numGoroutines * numItemsPerGoroutine
	assert.Len(t, mgr.AllItems(), expectedItems)
}
