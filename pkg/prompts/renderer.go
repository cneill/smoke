package prompts

import (
	"encoding/json"
	"strings"
)

// MarkdownRenderer renders a Prompt to Markdown.
type MarkdownRenderer struct{}

func (MarkdownRenderer) Render(p *Prompt) string {
	builder := &strings.Builder{}

	for _, sectionType := range p.order {
		sec := p.sections[sectionType]
		if sec == nil || len(sec.Nodes) == 0 {
			continue
		}
		// Section heading
		builder.WriteString("## ")
		builder.WriteString(string(sectionType))
		builder.WriteString("\n\n")

		for _, node := range sec.Nodes {
			renderMarkdownNode(builder, node, 0)
		}

		// Separate sections
		builder.WriteString("\n")
	}

	return builder.String()
}

func renderMarkdownNode(builder *strings.Builder, n Node, depth int) {
	switch value := n.(type) {
	case *Text:
		builder.WriteString(value.Content)
		builder.WriteString("\n\n")

	case *Heading:
		level := max(1, value.Level)
		// Section headings use '##'; nested headings start at '###' for level 1
		hashCount := min(6, 2+level)
		builder.WriteString(strings.Repeat("#", hashCount))
		builder.WriteByte(' ')
		builder.WriteString(value.Text)
		builder.WriteString("\n\n")

	case *CodeBlock:
		builder.WriteString("```")

		if value.Lang != "" {
			builder.WriteString(value.Lang)
		}

		builder.WriteString("\n")
		builder.WriteString(value.Code)

		if !strings.HasSuffix(value.Code, "\n") {
			builder.WriteString("\n")
		}

		builder.WriteString("```\n\n")

	case *BulletList:
		renderMarkdownList(builder, value.Items, depth)
		builder.WriteString("\n")

	default:
		// unknown node; ignore
	}
}

func renderMarkdownList(builder *strings.Builder, items []ListItem, depth int) {
	for _, it := range items {
		indent := strings.Repeat(" ", depth*4)
		builder.WriteString(indent)
		builder.WriteString("* ")
		builder.WriteString(it.Text)
		builder.WriteString("\n")

		if len(it.Children) > 0 {
			renderMarkdownList(builder, it.Children, depth+1)
		}
	}
}

// JSONRenderer renders a Prompt to a JSON-friendly map, plus convenience to get a string.
type JSONRenderer struct{}

func (JSONRenderer) RenderMap(prompt *Prompt) map[string]any {
	out := make(map[string]any)

	for _, sectionType := range prompt.order {
		section := prompt.sections[sectionType]
		if section == nil || len(section.Nodes) == 0 {
			continue
		}

		key := snakeCase(string(sectionType))

		var arr []any

		for _, node := range section.Nodes {
			switch value := node.(type) {
			case *Text:
				arr = append(arr, value.Content)
			case *Heading:
				arr = append(arr, map[string]any{"heading": value.Text, "level": value.Level})
			case *CodeBlock:
				arr = append(arr, map[string]any{"code": value.Code, "lang": value.Lang})
			case *BulletList:
				arr = append(arr, listItemsToJSON(value.Items))
			default:
				// leave as TODO for unknown node types
			}
		}

		out[key] = arr
	}

	return out
}

func listItemsToJSON(items []ListItem) any {
	if len(items) == 0 {
		return []any{}
	}

	arr := make([]any, 0, len(items))
	for _, item := range items {
		itemMap := map[string]any{"text": item.Text}
		if len(item.Children) > 0 {
			itemMap["children"] = listItemsToJSON(item.Children)
		}

		arr = append(arr, itemMap)
	}

	return arr
}

func (r JSONRenderer) RenderString(prompt *Prompt) string {
	promptMap := r.RenderMap(prompt)

	jsonBytes, err := json.Marshal(promptMap)
	if err != nil {
		return "{}"
	}

	return string(jsonBytes)
}
