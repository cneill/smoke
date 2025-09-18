package main

import (
	"strings"
)

// SectionType defines the canonical section names used when composing prompts.
// Order is defined by orderedSections() below.
type SectionType string

// order inspired by Anthropic - https://x.com/mattpocockuk/status/1958179930262356032/photo/1
const (
	SectionTask                SectionType = "Task"
	SectionTone                SectionType = "Tone"
	SectionBackground          SectionType = "Background"
	SectionDescription         SectionType = "Description"
	SectionRules               SectionType = "Rules"
	SectionExamples            SectionType = "Examples"
	SectionConversationHistory SectionType = "Conversation History"
	SectionInstructions        SectionType = "Instructions"
	SectionFormatting          SectionType = "Formatting"
)

func orderedSections() []SectionType {
	return []SectionType{
		SectionTask,
		SectionTone,
		SectionBackground,
		SectionDescription,
		SectionRules,
		SectionExamples,
		SectionConversationHistory,
		SectionInstructions,
		SectionFormatting,
	}
}

// Node is a content element in a section. Renderers will walk these nodes.
type Node interface{ isNode() }

// Text represents a paragraph or sentence.
type Text struct{ Content string }

func (*Text) isNode() {}

// Heading is an intra-section heading. Level 1 renders as ###, level 2 as ####, etc.
type Heading struct {
	Level int
	Text  string
}

func (*Heading) isNode() {}

// CodeBlock is a fenced code block with an optional language hint.
type CodeBlock struct{ Lang, Code string }

func (*CodeBlock) isNode() {}

// ListItem is an item in a BulletList, which may contain nested children.
type ListItem struct {
	Text     string
	Children []ListItem
}

// BulletList is a list of items, possibly nested.
type BulletList struct{ Items []ListItem }

func (*BulletList) isNode() {}

// Section groups nodes under a SectionType.
type Section struct {
	Type  SectionType
	Nodes []Node
}

// Prompt is a complete prompt with ordered sections.
type Prompt struct {
	Name     string
	sections map[SectionType]*Section
	order    []SectionType
}

// NewPrompt creates an empty prompt with canonical section ordering.
func NewPrompt(name string) *Prompt {
	return &Prompt{
		Name:     name,
		sections: make(map[SectionType]*Section),
		order:    orderedSections(),
	}
}

// Section returns the section for the given type, creating it if needed.
func (p *Prompt) Section(sectionType SectionType) *Section {
	if section, ok := p.sections[sectionType]; ok {
		return section
	}

	section := &Section{Type: sectionType}
	p.sections[sectionType] = section

	return section
}

// Clone returns a deep copy of the prompt and its sections/nodes.
func (p *Prompt) Clone() *Prompt {
	cp := &Prompt{
		Name:     p.Name,
		sections: make(map[SectionType]*Section, len(p.sections)),
		order:    append([]SectionType(nil), p.order...),
	}

	for sectionType, section := range p.sections {
		cp.sections[sectionType] = cloneSection(section)
	}

	return cp
}

func cloneSection(section *Section) *Section {
	if section == nil {
		return nil
	}

	out := &Section{Type: section.Type}
	for _, node := range section.Nodes {
		out.Nodes = append(out.Nodes, cloneNode(node))
	}

	return out
}

func cloneNode(node Node) Node {
	switch value := node.(type) {
	case *Text:
		cloned := *value
		return &cloned
	case *Heading:
		cloned := *value
		return &cloned
	case *CodeBlock:
		cloned := *value
		return &cloned
	case *BulletList:
		items := cloneListItems(value.Items)
		return &BulletList{Items: items}
	default:
		return node
	}
}

func cloneListItems(items []ListItem) []ListItem {
	if len(items) == 0 {
		return nil
	}

	out := make([]ListItem, len(items))
	for i := range items {
		out[i].Text = items[i].Text
		out[i].Children = cloneListItems(items[i].Children)
	}

	return out
}

// snakeCase converts a section name into snake_case for JSON keys.
func snakeCase(s string) string {
	lower := strings.ToLower(s)
	return strings.ReplaceAll(lower, " ", "_")
}
