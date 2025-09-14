package plan

import (
	"encoding/json"
	"fmt"
	"math/rand/v2"
	"slices"
	"time"
)

type ItemUnion struct {
	TaskItem       *TaskItem
	ContextItem    *ContextItem
	CompletionItem *CompletionItem
}

func (i *ItemUnion) OK() error {
	if i == nil {
		return fmt.Errorf("item is nil")
	}

	if i.Type() == ItemTypeUnknown {
		return fmt.Errorf("unknown item type, maybe more than one item specified")
	}

	return nil
}

func (i *ItemUnion) Type() ItemType {
	typ := ItemTypeUnknown

	if i == nil {
		return typ
	}

	if i.TaskItem != nil {
		typ = ItemTypeTask
	}

	if i.ContextItem != nil {
		if typ != ItemTypeUnknown {
			typ = ItemTypeUnknown
		} else {
			typ = ItemTypeContext
		}
	}

	if i.CompletionItem != nil {
		if typ != ItemTypeUnknown {
			typ = ItemTypeUnknown
		} else {
			typ = ItemTypeCompletion
		}
	}

	return typ
}

func (i *ItemUnion) Operation() Operation {
	operation := OperationAdd

	switch i.Type() { //nolint:exhaustive
	case ItemTypeTask:
		operation = i.TaskItem.Operation
	case ItemTypeContext:
		operation = i.ContextItem.Operation
	}

	return operation
}

func (i *ItemUnion) MarshalJSON() ([]byte, error) {
	var (
		bytes []byte
		err   error
	)

	switch i.Type() { //nolint:exhaustive
	case ItemTypeContext:
		bytes, err = json.Marshal(i.ContextItem)
	case ItemTypeCompletion:
		bytes, err = json.Marshal(i.CompletionItem)
	case ItemTypeTask:
		bytes, err = json.Marshal(i.TaskItem)
	default:
		err = fmt.Errorf("unknown item type: %q", i.Type())
	}

	return bytes, err
}

func (i *ItemUnion) UnmarshalJSON(data []byte) error {
	var itemTypeOnly struct {
		ItemType ItemType `json:"item_type"`
	}

	if err := json.Unmarshal(data, &itemTypeOnly); err != nil {
		return fmt.Errorf("failed to get item type: %w", err)
	}

	switch itemTypeOnly.ItemType { //nolint:exhaustive
	case ItemTypeContext:
		i.ContextItem = &ContextItem{}
		if err := json.Unmarshal(data, i.ContextItem); err != nil {
			return fmt.Errorf("failed to unmarshal context item: %w", err)
		}
	case ItemTypeCompletion:
		i.CompletionItem = &CompletionItem{}
		if err := json.Unmarshal(data, i.CompletionItem); err != nil {
			return fmt.Errorf("failed to unmarshal completion item: %w", err)
		}
	case ItemTypeTask:
		i.TaskItem = &TaskItem{}
		if err := json.Unmarshal(data, i.TaskItem); err != nil {
			return fmt.Errorf("failed to unmarshal task item: %w", err)
		}
	default:
		return fmt.Errorf("unknown item type: %q", itemTypeOnly.ItemType)
	}

	return nil
}

func (i *ItemUnion) ID() string {
	switch i.Type() { //nolint:exhaustive
	case ItemTypeCompletion:
		return i.CompletionItem.ID
	case ItemTypeContext:
		return i.ContextItem.ID
	case ItemTypeTask:
		return i.TaskItem.ID
	default:
		return ""
	}
}

type ItemType string

const (
	ItemTypeUnknown    ItemType = ""
	ItemTypeContext    ItemType = "context"
	ItemTypeCompletion ItemType = "completion"
	ItemTypeTask       ItemType = "task"
)

type Operation string

const (
	OperationAdd    Operation = "add"
	OperationUpdate Operation = "update"
)

type BaseItem struct {
	ID        string    `json:"id"`
	Time      time.Time `json:"time"`
	ItemType  ItemType  `json:"item_type"`
	Operation Operation `json:"operation"`
}

func NewBaseItem(itemType ItemType) *BaseItem {
	return &BaseItem{
		ID:       randID(),
		Time:     time.Now(),
		ItemType: itemType,
	}
}

type TaskItem struct {
	*BaseItem

	Content      string   `json:"content"`
	Parent       string   `json:"parent"`
	Dependencies []string `json:"dependencies"`
}

func NewTaskItem(content string) *TaskItem {
	return &TaskItem{
		BaseItem:     NewBaseItem(ItemTypeTask),
		Content:      content,
		Dependencies: []string{},
	}
}

func (t *TaskItem) SetID(id string) *TaskItem {
	t.ID = id
	return t
}

func (t *TaskItem) SetOperation(operation Operation) *TaskItem {
	t.Operation = operation
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

type ContextType string

const (
	ContextTypeCode       ContextType = "code"
	ContextTypeDecision   ContextType = "decision"
	ContextTypeReference  ContextType = "reference"
	ContextTypeConstraint ContextType = "constraint"
)

type ContextItem struct {
	*BaseItem

	ContextType ContextType `json:"context_type"`
	Content     string      `json:"content"`
	Owners      []string    `json:"owners"`
}

func NewContextItem(contextType ContextType, content string) *ContextItem {
	return &ContextItem{
		BaseItem: NewBaseItem(ItemTypeContext),

		ContextType: contextType,
		Content:     content,
		Owners:      []string{},
	}
}

func (c *ContextItem) SetID(id string) *ContextItem {
	c.ID = id
	return c
}

func (c *ContextItem) SetOperation(operation Operation) *ContextItem {
	c.Operation = operation
	return c
}

func (c *ContextItem) SetOwners(ownerIDs ...string) *ContextItem {
	c.Owners = ownerIDs
	return c
}

type CompletionStatus string

const (
	CompletionStatusUnknown  CompletionStatus = ""
	CompletionStatusSuccess  CompletionStatus = "success"
	CompletionStatusFailed   CompletionStatus = "failed"
	CompletionStatusPartial  CompletionStatus = "partial"
	CompletionStatusObsolete CompletionStatus = "obsolete"
)

type CompletionItem struct {
	*BaseItem

	Content string           `json:"content"`
	Status  CompletionStatus `json:"status"`
	TaskIDs []string         `json:"task_ids"`
}

func NewCompletionItem(content string, taskIDs ...string) *CompletionItem {
	return &CompletionItem{
		BaseItem: NewBaseItem(ItemTypeCompletion),

		Content: content,
		Status:  CompletionStatusSuccess,
		TaskIDs: taskIDs,
	}
}

func (c *CompletionItem) SetStatus(status CompletionStatus) *CompletionItem {
	c.Status = status
	return c
}

const idChars = "abcdef0123456789"

// randID returns a random 32-character hex string
// TODO: consolidate with llms.randID?
func randID() string {
	output := []byte{}
	for range 32 {
		output = append(output, idChars[rand.IntN(len(idChars))]) //nolint:gosec
	}

	return string(output)
}
