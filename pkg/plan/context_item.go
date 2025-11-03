package plan

import (
	"fmt"
	"slices"
)

type ContextType string

const (
	ContextTypeUnknown    ContextType = ""
	ContextTypeCode       ContextType = "code"
	ContextTypeDecision   ContextType = "decision"
	ContextTypeReference  ContextType = "reference"
	ContextTypeConstraint ContextType = "constraint"
	ContextTypeConvention ContextType = "convention"
)

func allContextTypes() []ContextType {
	return []ContextType{
		ContextTypeCode, ContextTypeDecision, ContextTypeReference, ContextTypeConstraint, ContextTypeConvention,
	}
}

type ContextItem struct {
	*BaseItem

	ContextType ContextType `json:"context_type"`
	Content     string      `json:"content"`
	Owners      []string    `json:"owners"`
}

func NewContextItem(contextType ContextType, content string, operation Operation) *ContextItem {
	return &ContextItem{
		BaseItem: NewBaseItem(ItemTypeContext, operation),

		ContextType: contextType,
		Content:     content,
		Owners:      []string{},
	}
}

func (c *ContextItem) OK() error {
	switch {
	case c == nil:
		return fmt.Errorf("context item is empty")
	case c.BaseItem.OK() != nil:
		return fmt.Errorf("error with base properties: %w", c.BaseItem.OK())
	case c.ItemType != ItemTypeContext:
		return fmt.Errorf("mismatched item type: expecting %q, got %q", ItemTypeContext, c.ItemType)
	case !slices.Contains(allContextTypes(), c.ContextType):
		return fmt.Errorf("%w: %q", ErrUnknownContextType, c.ContextType)
	case c.Content == "":
		return fmt.Errorf("missing content")
		// case len(c.Owners) == 0:
		// 	return fmt.Errorf("missing owners")
	}

	return nil
}

func (c *ContextItem) SetID(id string) *ContextItem {
	c.ID = id
	return c
}

func (c *ContextItem) SetOwners(ownerIDs ...string) *ContextItem {
	c.Owners = ownerIDs
	return c
}

func (c *ContextItem) ToUnion() *ItemUnion {
	return &ItemUnion{ContextItem: c}
}
