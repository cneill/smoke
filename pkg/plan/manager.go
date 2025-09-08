package plan

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"sync"
)

type Manager struct {
	writer    io.Writer
	IsWriting bool

	items     []*ItemUnion
	completed map[string]CompletionStatus
	mutex     sync.RWMutex
}

func NewManager(writer io.Writer) *Manager {
	return &Manager{
		writer:    writer,
		IsWriting: true,
		items:     []*ItemUnion{},
		completed: map[string]CompletionStatus{},
		mutex:     sync.RWMutex{},
	}
}

func ManagerFromReader(reader io.ReadWriter) (*Manager, error) {
	manager := NewManager(reader)
	manager.IsWriting = false

	scanner := bufio.NewScanner(reader)
	itemNumber := 1

	for scanner.Scan() {
		item := &ItemUnion{}

		if err := json.Unmarshal(scanner.Bytes(), item); err != nil {
			return nil, fmt.Errorf("failed to parse item number %d: %w", itemNumber, err)
		}

		if err := manager.AddItem(item); err != nil {
			return nil, fmt.Errorf("failed to add iten number %d: %w", itemNumber, err)
		}

		itemNumber++
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("failed to read log: %w", err)
	}

	manager.IsWriting = true

	return manager, nil
}

func (m *Manager) AddItem(item *ItemUnion) error {
	if err := item.OK(); err != nil {
		return fmt.Errorf("item error: %w", err)
	}

	if item.Type() == ItemTypeTask {
		if err := m.validateTaskDependencies(item.TaskItem); err != nil {
			return fmt.Errorf("failed to validate task item dependencies: %w", err)
		}
	}

	m.mutex.Lock()
	defer m.mutex.Unlock()

	m.items = append(m.items, item)

	// if we're e.g. loading an existing log with ManagerFromReader, don't write the same items again
	if m.IsWriting {
		marshalled, err := json.Marshal(item)
		if err != nil {
			return fmt.Errorf("failed to marshal item to JSON: %w", err)
		}

		marshalled = append(marshalled, '\n')

		if _, err := m.writer.Write(marshalled); err != nil {
			return fmt.Errorf("failed to write item JSON: %w", err)
		}
	}

	// Mark previous items complete
	if item.Type() == ItemTypeCompletion {
		for _, taskID := range item.CompletionItem.TaskIDs {
			if _, ok := m.completed[taskID]; ok {
				slog.Warn("completed task marked complete again", "id", taskID)
			}

			m.completed[taskID] = item.CompletionItem.Status
		}
	}

	return nil
}

func (m *Manager) AllItems() []*ItemUnion {
	return m.items
}

func (m *Manager) Completed() map[string]CompletionStatus {
	return m.completed
}

func (m *Manager) GetItemByID(searchID string) *ItemUnion {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	for _, item := range m.items {
		if item.ID() == searchID {
			return item
		}
	}

	return nil
}

// GetPendingTasks returns all tasks that are not marked completed OR that are marked failed / partially completed
func (m *Manager) GetPendingTasks() []*TaskItem {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	pending := []*TaskItem{}

	for _, item := range m.items {
		if item.Type() != ItemTypeTask {
			continue
		}

		status, exists := m.completed[item.TaskItem.ID]
		if !exists || status != CompletionStatusSuccess {
			pending = append(pending, item.TaskItem)
		}
	}

	return pending
}

// GetContextFor searches all ContextItems and returns the ones with owners matching 'searchID'.
func (m *Manager) GetContextFor(searchID string) []*ContextItem {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	allContext := []*ContextItem{}

	for _, item := range m.items {
		if item.Type() != ItemTypeContext {
			continue
		}

		for _, ownerID := range item.ContextItem.Owners {
			if ownerID == searchID {
				allContext = append(allContext, item.ContextItem)
			}
		}
	}

	return allContext
}

// validateTaskDependencies ensures that we have matching items for the dependencies declared on a TaskItem
func (m *Manager) validateTaskDependencies(item *TaskItem) error {
	missingDependencyIDs := []string{}

	for _, dependencyID := range item.Dependencies {
		if item := m.GetItemByID(dependencyID); item == nil {
			missingDependencyIDs = append(missingDependencyIDs, dependencyID)
		}
	}

	if len(missingDependencyIDs) > 0 {
		return fmt.Errorf("unknown dependency IDs: %s", strings.Join(missingDependencyIDs, ", "))
	}

	return nil
}
