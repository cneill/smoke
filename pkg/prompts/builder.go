package prompts

import "fmt"

// Builder provides a fluent API for constructing prompts.
type Builder struct {
	p *Prompt
}

func NewBuilder(name string) *Builder {
	return &Builder{p: NewPrompt(name)}
}

func (b *Builder) Build() *Prompt { return b.p }

// Add appends nodes to a section.
func (b *Builder) Add(t SectionType, nodes ...Node) *Builder {
	s := b.p.Section(t)
	s.Nodes = append(s.Nodes, nodes...)

	return b
}

// Prepend adds nodes to the beginning of a section.
func (b *Builder) Prepend(t SectionType, nodes ...Node) *Builder {
	s := b.p.Section(t)
	s.Nodes = append(append(Nodes(nil), nodes...), s.Nodes...)

	return b
}

// Replace sets the nodes for a section.
func (b *Builder) Replace(t SectionType, nodes ...Node) *Builder {
	b.p.sections[t] = &Section{Type: t, Nodes: append(Nodes(nil), nodes...)}
	return b
}

// Remove deletes a section entirely.
func (b *Builder) Remove(t SectionType) *Builder {
	delete(b.p.sections, t)
	return b
}

// Convenience constructors
func P(text string) *Text {
	return &Text{Content: text}
}

func Pf(format string, arguments ...any) *Text {
	return &Text{Content: fmt.Sprintf(format, arguments...)}
}

func H(level int, text string) *Heading {
	return &Heading{
		Level: level,
		Text:  text,
	}
}

func Hf(level int, format string, arguments ...any) *Heading {
	return &Heading{
		Level: level,
		Text:  fmt.Sprintf(format, arguments...),
	}
}

func Code(lang, code string) *CodeBlock {
	return &CodeBlock{
		Lang: lang,
		Code: code,
	}
}

func Item(text string, children ...ListItem) ListItem {
	return ListItem{
		Text:     text,
		Children: append([]ListItem(nil), children...),
	}
}

func Itemf(format string, arguments ...any) ListItem {
	return ListItem{
		Text:     fmt.Sprintf(format, arguments...),
		Children: []ListItem{},
	}
}

func List(items ...ListItem) *BulletList {
	return &BulletList{Items: append([]ListItem(nil), items...)}
}

// Presets and merging

type MergeStrategy int

const (
	Append MergeStrategy = iota
	Replace
	SkipExisting
)

type Preset struct {
	Name     string
	Sections map[SectionType]Nodes
}

func (b *Builder) ApplyPreset(preset Preset, strategy MergeStrategy) *Builder {
	for sectionType, nodes := range preset.Sections {
		existing, ok := b.p.sections[sectionType]

		switch strategy {
		case Replace:
			b.p.sections[sectionType] = &Section{Type: sectionType, Nodes: nodes.Clone()}
		case SkipExisting:
			if ok && len(existing.Nodes) > 0 {
				continue
			}

			fallthrough
		case Append:
			if !ok {
				existing = &Section{Type: sectionType}
			}

			existing.Nodes = append(existing.Nodes, nodes.Clone()...)
			b.p.sections[sectionType] = existing
		}
	}

	return b
}
