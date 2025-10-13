package plan

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
)

type Manager struct {
	IsWriting   bool
	writer      io.Writer
	writerMutex sync.Mutex

	items     []*ItemUnion
	completed map[string]CompletionStatus // completed contains the Item's ID and its current (final) status
	updated   map[string]int              // updated contains the ID and the index of the latest updated version
	itemMutex sync.RWMutex
}

func NewManager(writer io.Writer) *Manager {
	return &Manager{
		IsWriting:   true,
		writer:      writer,
		writerMutex: sync.Mutex{},

		items:     []*ItemUnion{},
		completed: map[string]CompletionStatus{},
		updated:   map[string]int{},
		itemMutex: sync.RWMutex{},
	}
}

// ManagerFromPath manages the file system aspects of loading/creating a new plan file. Must supply an absolute path
// here.
func ManagerFromPath(path string) (*Manager, error) {
	var manager *Manager

	if !filepath.IsAbs(path) {
		return nil, fmt.Errorf("plan file %q is not an absolute path", path)
	}

	_, statErr := os.Stat(path)
	switch {
	case statErr != nil && !errors.Is(statErr, fs.ErrNotExist):
		return nil, fmt.Errorf("error opening existing plan file %q: %w", path, statErr)
	case errors.Is(statErr, fs.ErrNotExist):
		planFile, openErr := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
		if openErr != nil {
			return nil, fmt.Errorf("failed to create plan file %q: %w", path, openErr)
		}

		slog.Debug("created new plan file", "path", path)

		manager = NewManager(planFile)
	default:
		planFile, openErr := os.OpenFile(path, os.O_APPEND|os.O_RDWR, 0o644)
		if openErr != nil {
			return nil, fmt.Errorf("failed to open plan file %q: %w", path, openErr)
		}

		fromReader, readErr := ManagerFromReader(planFile)
		if readErr != nil {
			return nil, fmt.Errorf("failed to read existing plan file: %w", readErr)
		}

		slog.Debug("opened and parsed existing plan file", "path", path)

		manager = fromReader
	}

	return manager, nil
}

// ManagerFromReader takes an [io.ReadWriter] and creates a Manager from its existing contents, then returns it ready
// for writing.
func ManagerFromReader(reader io.ReadWriter) (*Manager, error) {
	manager := NewManager(reader)
	manager.IsWriting = false

	scanner := bufio.NewScanner(reader)
	itemNumber := 1

	for scanner.Scan() {
		contents := scanner.Bytes()
		item := &ItemUnion{}

		// Skip empty lines
		if len(bytes.TrimSpace(contents)) == 0 {
			continue
		}

		if err := json.Unmarshal(contents, item); err != nil {
			return nil, fmt.Errorf("failed to parse item number %d: %w", itemNumber, err)
		}

		if err := manager.HandleItem(item); err != nil {
			return nil, fmt.Errorf("failed to handle item number %d: %w", itemNumber, err)
		}

		itemNumber++
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("failed to read log: %w", err)
	}

	manager.IsWriting = true

	return manager, nil
}

func (m *Manager) HandleItem(item *ItemUnion) error {
	operation := item.Operation()

	switch operation { //nolint:exhaustive
	case OperationAdd:
		return m.AddItem(item)
	case OperationUpdate:
		return m.UpdateItem(item)
	}

	return fmt.Errorf("unknown or empty item operation: %q", operation)
}

func (m *Manager) AddItem(item *ItemUnion) error {
	if err := item.OK(); err != nil {
		return fmt.Errorf("item error: %w", err)
	}

	if op := item.Operation(); op != OperationAdd {
		return fmt.Errorf("expecting %q operation, got %q", OperationAdd, op)
	}

	if item.Type() == ItemTypeTask {
		if err := m.validateTaskDependencies(item.TaskItem); err != nil {
			return fmt.Errorf("failed to validate task item dependencies: %w", err)
		}
	}

	if existing := m.GetItemByID(item.ID()); existing != nil {
		return fmt.Errorf("item already exists for ID %q, refusing to add duplicate", item.ID())
	}

	m.itemMutex.Lock()

	m.items = append(m.items, item)

	// Mark previous items complete
	if item.Type() == ItemTypeCompletion {
		for _, taskID := range item.CompletionItem.TaskIDs {
			if _, ok := m.completed[taskID]; ok {
				slog.Warn("completed task marked complete again", "id", taskID)
			}

			m.completed[taskID] = item.CompletionItem.Status
		}
	}

	m.itemMutex.Unlock()

	if err := m.writeItem(item); err != nil {
		return err
	}

	return nil
}

func (m *Manager) UpdateItem(item *ItemUnion) error {
	if err := item.OK(); err != nil {
		return fmt.Errorf("item error: %w", err)
	}

	if op := item.Operation(); op != OperationUpdate {
		return fmt.Errorf("expecting %q operation, got %q", OperationUpdate, op)
	}

	// TODO: always forbid this?
	// if item.Type() == ItemTypeCompletion {
	// 	return fmt.Errorf("can't update completion item")
	// }

	if item.Type() == ItemTypeTask {
		if err := m.validateTaskDependencies(item.TaskItem); err != nil {
			return fmt.Errorf("failed to validate task item dependencies: %w", err)
		}
	}

	if existing := m.GetItemByID(item.ID()); existing == nil {
		return fmt.Errorf("item with ID %q does not exist to update", item.ID())
	}

	m.itemMutex.Lock()
	defer m.itemMutex.Unlock()

	m.items = append(m.items, item)
	m.updated[item.ID()] = len(m.items) - 1

	if err := m.writeItem(item); err != nil {
		return err
	}

	return nil
}

func (m *Manager) AllItems() []*ItemUnion {
	return m.items
}

// Completed returns a map of all tasks marked completed. These may have been partial / failed completions.
func (m *Manager) Completed() map[string]CompletionStatus {
	return m.completed
}

func (m *Manager) GetItemByID(searchID string) *ItemUnion {
	m.itemMutex.RLock()
	defer m.itemMutex.RUnlock()

	for idx, item := range m.items {
		if item.ID() == searchID {
			// skip old versions of this item if it has been updated by a subsequent item
			if revisedIdx, ok := m.updated[item.ID()]; ok && revisedIdx > idx {
				continue
			}

			return item
		}
	}

	return nil
}

// GetPendingTasks returns all tasks that are not marked completed OR that are marked failed / partially completed.
// Skips old versions of revised items.
func (m *Manager) GetPendingTasks() []*TaskItem {
	m.itemMutex.RLock()
	defer m.itemMutex.RUnlock()

	pending := []*TaskItem{}

	for idx, item := range m.items {
		if item.Type() != ItemTypeTask {
			continue
		}

		// skip old versions if this task has been updated by a subsequent task item
		if revisedIdx, ok := m.updated[item.ID()]; ok && revisedIdx > idx {
			continue
		}

		// skip tasks completed successfully
		status, exists := m.completed[item.TaskItem.ID]
		if exists && status == CompletionStatusSuccess {
			continue
		}

		pending = append(pending, item.TaskItem)
	}

	return pending
}

// GetContextFor searches all ContextItems and returns the ones with owners matching 'searchID'.
func (m *Manager) GetContextFor(searchID string) []*ContextItem {
	m.itemMutex.RLock()
	defer m.itemMutex.RUnlock()

	allContext := []*ContextItem{}

	for idx, item := range m.items {
		if item.Type() != ItemTypeContext {
			continue
		}

		// skip old versions if this context item has been updated by a subsequent context item
		if revisedIdx, ok := m.updated[item.ID()]; ok && revisedIdx > idx {
			continue
		}

		if slices.Contains(item.ContextItem.Owners, searchID) {
			allContext = append(allContext, item.ContextItem)
		}
	}

	return allContext
}

func (m *Manager) Teardown() error {
	m.writerMutex.Lock()
	defer m.writerMutex.Unlock()

	m.IsWriting = false

	if closer, ok := m.writer.(io.WriteCloser); ok {
		if err := closer.Close(); err != nil {
			return fmt.Errorf("failed to close plan manager writer: %w", err)
		}
	}

	return nil
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

func (m *Manager) writeItem(item *ItemUnion) error {
	if !m.IsWriting {
		// if we're e.g. loading an existing log with ManagerFromReader, don't write the same items again
		return nil
	}

	marshalled, err := json.Marshal(item)
	if err != nil {
		return fmt.Errorf("failed to marshal item to JSON: %w", err)
	}

	marshalled = append(marshalled, '\n')

	m.writerMutex.Lock()
	defer m.writerMutex.Unlock()

	if _, err := m.writer.Write(marshalled); err != nil {
		return fmt.Errorf("failed to write item JSON: %w", err)
	}

	return nil
}
