// Package prompts contains prompts used to interact with the LLMs, such as the overall system prompt that describes how
// the model should respond to questions or requests for code changes.
package prompts

import (
	"io"
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
type Node interface {
	Clone() Node
	RenderMarkdown(builder *strings.Builder, depth int)
	// TODO: handle JSON
}

type Nodes []Node

func (n Nodes) Clone() Nodes {
	result := make(Nodes, len(n))
	for i := range n {
		result[i] = n[i].Clone()
	}

	return result
}

type Writer interface {
	io.StringWriter
}

// Text represents a paragraph or sentence.
type Text struct {
	Content string
}

func (t *Text) Clone() Node {
	temp := *t
	return &temp
}

func (t *Text) RenderMarkdown(builder *strings.Builder, _ int) {
	builder.WriteString(t.Content)
	builder.WriteString("\n\n")
}

// Heading is an intra-section heading. Level 1 renders as ###, level 2 as ####, etc.
type Heading struct {
	Level int
	Text  string
}

func (h *Heading) Clone() Node {
	temp := *h
	return &temp
}

func (h *Heading) RenderMarkdown(builder *strings.Builder, _ int) {
	level := max(1, h.Level)
	// Section headings use '##'; nested headings start at '###' for level 1
	hashCount := min(6, 2+level)
	builder.WriteString(strings.Repeat("#", hashCount))
	builder.WriteString(" ")
	builder.WriteString(h.Text)
	builder.WriteString("\n\n")
}

// CodeBlock is a fenced code block with an optional language hint.
type CodeBlock struct {
	Lang string
	Code string
}

func (c *CodeBlock) Clone() Node {
	temp := *c
	return &temp
}

func (c *CodeBlock) RenderMarkdown(builder *strings.Builder, _ int) {
	builder.WriteString("```")

	if c.Lang != "" {
		builder.WriteString(c.Lang)
	}

	builder.WriteString("\n")
	builder.WriteString(c.Code)

	if !strings.HasSuffix(c.Code, "\n") {
		builder.WriteString("\n")
	}

	builder.WriteString("```\n\n")
}

// ListItem is an item in a BulletList, which may contain nested children.
type ListItem struct {
	Text     string
	Children ListItems
}

type ListItems []ListItem

func (l ListItems) Clone() ListItems {
	if len(l) == 0 {
		return nil
	}

	out := make(ListItems, len(l))
	for i := range l {
		out[i].Text = l[i].Text
		out[i].Children = l[i].Children.Clone()
	}

	return out
}

func (l ListItems) RenderMarkdown(builder *strings.Builder, depth int) {
	for _, item := range l {
		indent := strings.Repeat(" ", depth*4)
		builder.WriteString(indent)
		builder.WriteString("* ")
		builder.WriteString(item.Text)
		builder.WriteString("\n")

		if len(item.Children) > 0 {
			item.Children.RenderMarkdown(builder, depth+1)
		}
	}
}

// BulletList is a list of items, possibly nested.
type BulletList struct {
	Items ListItems
}

func (b *BulletList) Clone() Node {
	return &BulletList{Items: b.Items.Clone()}
}

func (b *BulletList) RenderMarkdown(builder *strings.Builder, depth int) {
	b.Items.RenderMarkdown(builder, depth)
	builder.WriteString("\n")
}

// Section groups nodes under a SectionType.
type Section struct {
	Type  SectionType
	Nodes Nodes
}

func (s *Section) Clone() *Section {
	if s == nil {
		return nil
	}

	return &Section{Type: s.Type, Nodes: s.Nodes.Clone()}
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

func (p *Prompt) Markdown() string {
	renderer := MarkdownRenderer{}
	return renderer.Render(p)
}

func (p *Prompt) JSON() string {
	renderer := JSONRenderer{}
	return renderer.RenderString(p)
}

func cloneSection(section *Section) *Section {
	if section == nil {
		return nil
	}

	out := &Section{Type: section.Type}
	for _, node := range section.Nodes {
		out.Nodes = append(out.Nodes, node.Clone())
	}

	return out
}

// snakeCase converts a section name into snake_case for JSON keys.
func snakeCase(s string) string {
	lower := strings.ToLower(s)
	return strings.ReplaceAll(lower, " ", "_")
}
