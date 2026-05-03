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
			node.RenderMarkdown(builder, 0)
		}

		// Separate sections
		builder.WriteString("\n")
	}

	return builder.String()
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
