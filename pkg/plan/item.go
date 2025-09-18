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
		return fmt.Errorf("%w, maybe more than one item specified in union", ErrUnknownItemType)
	}

	if i.Operation() == OperationUnknown {
		return ErrUnknownOperation
	}

	switch i.Type() { //nolint:exhaustive
	case ItemTypeCompletion:
		if err := i.CompletionItem.OK(); err != nil {
			return fmt.Errorf("%w: %w", ErrInvalidCompletionItem, err)
		}
	case ItemTypeContext:
		if err := i.ContextItem.OK(); err != nil {
			return fmt.Errorf("%w: %w", ErrInvalidContextItem, err)
		}
	case ItemTypeTask:
		if err := i.TaskItem.OK(); err != nil {
			return fmt.Errorf("%w: %w", ErrInvalidTaskItem, err)
		}
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
	operation := OperationUnknown

	switch i.Type() { //nolint:exhaustive
	case ItemTypeTask:
		operation = i.TaskItem.Operation
	case ItemTypeContext:
		operation = i.ContextItem.Operation
	case ItemTypeCompletion:
		operation = i.CompletionItem.Operation
	}

	if !slices.Contains(allOperations(), operation) {
		operation = OperationUnknown
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
		return fmt.Errorf("%w: failed to get item type: %w", ErrInvalidJSON, err)
	}

	switch itemTypeOnly.ItemType { //nolint:exhaustive
	case ItemTypeContext:
		i.ContextItem = &ContextItem{}
		if err := json.Unmarshal(data, i.ContextItem); err != nil {
			return fmt.Errorf("%w: %w", ErrInvalidContextItem, err)
		}
	case ItemTypeCompletion:
		i.CompletionItem = &CompletionItem{}
		if err := json.Unmarshal(data, i.CompletionItem); err != nil {
			return fmt.Errorf("%w: %w", ErrInvalidCompletionItem, err)
		}
	case ItemTypeTask:
		i.TaskItem = &TaskItem{}
		if err := json.Unmarshal(data, i.TaskItem); err != nil {
			return fmt.Errorf("%w: %w", ErrInvalidTaskItem, err)
		}
	default:
		return fmt.Errorf("%w: %q", ErrUnknownItemType, itemTypeOnly.ItemType)
	}

	if err := i.OK(); err != nil {
		switch i.Type() { //nolint:exhaustive
		case ItemTypeContext:
			return fmt.Errorf("%w: %w", ErrInvalidContextItem, err)
		case ItemTypeCompletion:
			return fmt.Errorf("%w: %w", ErrInvalidCompletionItem, err)
		case ItemTypeTask:
			return fmt.Errorf("%w: %w", ErrInvalidTaskItem, err)
		}
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
	OperationUnknown Operation = ""
	OperationAdd     Operation = "add"
	OperationUpdate  Operation = "update"
)

func allOperations() []Operation {
	return []Operation{
		OperationAdd,
		OperationUpdate,
	}
}

type BaseItem struct {
	ID        string    `json:"id"`
	Time      time.Time `json:"time"`
	ItemType  ItemType  `json:"item_type"`
	Operation Operation `json:"operation"`
}

func NewBaseItem(itemType ItemType, operation Operation) *BaseItem {
	return &BaseItem{
		ID:        RandID(),
		Time:      time.Now(),
		ItemType:  itemType,
		Operation: operation,
	}
}

func (b *BaseItem) OK() error {
	switch {
	case b == nil:
		return fmt.Errorf("missing base item properties")
	case b.ID == "":
		return fmt.Errorf("missing ID")
	case b.ItemType == ItemTypeUnknown:
		return ErrUnknownItemType
	case !slices.Contains(allOperations(), b.Operation):
		return ErrUnknownOperation
	}

	return nil
}

const idChars = "abcdef0123456789"

// RandID returns a random 32-character hex string
// TODO: consolidate with llms.RandID?
func RandID() string {
	output := []byte{}
	for range 32 {
		output = append(output, idChars[rand.IntN(len(idChars))]) //nolint:gosec
	}

	return string(output)
}
