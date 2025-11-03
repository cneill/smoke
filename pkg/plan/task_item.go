package plan

import (
	"fmt"
	"slices"
)

type TaskItem struct {
	*BaseItem

	Content      string   `json:"content"`
	Parent       string   `json:"parent"`
	Dependencies []string `json:"dependencies"`
}

func NewTaskItem(id, content string, operation Operation) *TaskItem {
	baseItem := NewBaseItem(ItemTypeTask, operation)
	baseItem.ID = id

	return &TaskItem{
		BaseItem:     baseItem,
		Content:      content,
		Dependencies: []string{},
	}
}

func (t *TaskItem) OK() error {
	switch {
	case t == nil:
		return fmt.Errorf("task item is empty")
	case t.BaseItem.OK() != nil:
		return fmt.Errorf("error with base properties: %w", t.BaseItem.OK())
	case t.ItemType != ItemTypeTask:
		return fmt.Errorf("mismatched item type: expecting %q, got %q", ItemTypeTask, t.ItemType)
	case t.Content == "":
		return fmt.Errorf("missing content")
	}

	return nil
}

func (t *TaskItem) SetID(id string) *TaskItem {
	t.ID = id
	return t
}

func (t *TaskItem) SetParent(parentID string) *TaskItem {
	t.Parent = parentID
	return t
}

func (t *TaskItem) SetDependencies(dependencyIDs ...string) *TaskItem {
	t.Dependencies = dependencyIDs
	return t
}

func (t *TaskItem) IsChildOf(taskID string) bool {
	return t.Parent == taskID
}

func (t *TaskItem) HasDependency(taskID string) bool {
	return slices.Contains(t.Dependencies, taskID)
}

func (t *TaskItem) ToUnion() *ItemUnion {
	return &ItemUnion{TaskItem: t}
}
