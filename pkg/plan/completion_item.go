package plan

import (
	"fmt"
	"slices"
)

type CompletionStatus string

const (
	CompletionStatusUnknown  CompletionStatus = ""
	CompletionStatusSuccess  CompletionStatus = "success"
	CompletionStatusFailed   CompletionStatus = "failed"
	CompletionStatusPartial  CompletionStatus = "partial"
	CompletionStatusObsolete CompletionStatus = "obsolete"
)

func allCompletionStatuses() []CompletionStatus {
	return []CompletionStatus{
		CompletionStatusSuccess, CompletionStatusFailed, CompletionStatusPartial, CompletionStatusObsolete,
	}
}

type CompletionItem struct {
	*BaseItem

	Content string           `json:"content"`
	Status  CompletionStatus `json:"status"`
	TaskIDs []string         `json:"task_ids"`
}

func NewCompletionItem(content string, taskIDs ...string) *CompletionItem {
	return &CompletionItem{
		BaseItem: NewBaseItem(ItemTypeCompletion, OperationAdd),

		Status:  CompletionStatusSuccess,
		Content: content,
		TaskIDs: taskIDs,
	}
}

func (c *CompletionItem) OK() error {
	switch {
	case c == nil:
		return fmt.Errorf("completion item is empty")
	case c.BaseItem.OK() != nil:
		return fmt.Errorf("error with base properties: %w", c.BaseItem.OK())
	case c.ItemType != ItemTypeCompletion:
		return fmt.Errorf("mismatched item type: expecting %q, got %q", ItemTypeCompletion, c.ItemType)
	case !slices.Contains(allCompletionStatuses(), c.Status):
		return fmt.Errorf("%w: %q", ErrUnknownCompletionStatus, c.Status)
	case c.Content == "":
		return fmt.Errorf("missing content")
	case len(c.TaskIDs) == 0:
		return fmt.Errorf("missing task IDs")
	}

	return nil
}

func (c *CompletionItem) SetStatus(status CompletionStatus) *CompletionItem {
	c.Status = status
	return c
}

func (c *CompletionItem) ToUnion() *ItemUnion {
	return &ItemUnion{CompletionItem: c}
}
