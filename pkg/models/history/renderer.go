package history

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
	"github.com/cneill/smoke/internal/uimsg"
	"github.com/muesli/reflow/wordwrap"
)

type Renderer struct {
	styles     Styles
	width      int
	mdRenderer *glamour.TermRenderer
}

func NewRenderer(width int) (*Renderer, error) {
	mdRenderer, err := getGlamourRenderer(width)
	if err != nil {
		return nil, err
	}

	return &Renderer{
		styles:     InitStyles(width),
		width:      width,
		mdRenderer: mdRenderer,
	}, nil
}

func (r *Renderer) Resize(width int) {
	r.width = width
	r.styles = InitStyles(width)
	// Rebuilding the markdown renderer on every resize is currently too expensive.
	// TODO: figure out how to make this more performant and revisit.
}

func (r *Renderer) RenderBubble(bubble Bubble) string {
	var sb strings.Builder

	headerParts := []string{bubble.Style.Title.Render(bubble.TitleText)}
	if bubble.SubtitleText != "" {
		headerParts = append(headerParts, bubble.Style.Subtitle.Render(bubble.SubtitleText))
	}

	header := bubble.Style.Container.Render(strings.Join(headerParts, "\n"))

	fmt.Fprintln(&sb, header)
	fmt.Fprintln(&sb, r.renderBubbleContent(bubble.Content))

	return sb.String()
}

func (r *Renderer) Styles() Styles {
	// TODO: stop tossing this between bubble creation and renderer - handle in one place
	return r.styles
}

func (r *Renderer) renderBubbleContent(content BubbleContent) string {
	contentWidth := r.contentWidth()

	switch content.kind {
	case bubbleContentEmpty:
		return ""
	case bubbleContentStructured:
		return r.renderStructuredHistoryContent(content.structured, contentWidth)
	case bubbleContentMarkdown:
		rendered := content.text
		if renderedMarkdown, err := r.mdRenderer.Render(rendered); err == nil {
			rendered = renderedMarkdown
		}

		return rendered
	case bubbleContentPlainText:
		fallthrough
	default:
		return wordwrap.String(content.text, contentWidth)
	}
}

func (r *Renderer) contentWidth() int {
	if r.width > 0 {
		return r.width
	}

	return defaultBubbleWidth
}

func (r *Renderer) renderStructuredHistoryContent(content *uimsg.HistoryContent, width int) string {
	parts := make([]string, 0, len(content.Blocks))

	for _, block := range content.Blocks {
		rendered := r.renderHistoryBlock(block, width)
		if strings.TrimSpace(rendered) == "" {
			continue
		}

		parts = append(parts, rendered)
	}

	return strings.Join(parts, "\n\n")
}

func (r *Renderer) renderHistoryBlock(block uimsg.HistoryBlock, width int) string {
	switch block.Type {
	case uimsg.HistoryBlockFields:
		return r.renderHistoryFieldsBlock(block, width)
	case uimsg.HistoryBlockMarkdown:
		return r.renderHistoryMarkdownBlock(block, width)
	case uimsg.HistoryBlockText:
		fallthrough
	default:
		return r.renderHistoryTextBlock(block, width)
	}
}

// TODO: all of these rely on the CommandBubble style because structured content originated with command output. Revisit
// this when fixing styling generally.
func (r *Renderer) renderHistoryFieldsBlock(block uimsg.HistoryBlock, width int) string {
	parts := make([]string, 0, len(block.Fields)+1)
	if block.Title != "" {
		parts = append(parts, r.styles.Command.SectionTitle.Render(block.Title))
	}

	for _, field := range block.Fields {
		parts = append(parts, r.styles.Command.RenderField(field.Label, field.Value, width))
	}

	return bubbleContentStyle(block, r.styles.CommandBubble.Content).Render(strings.Join(parts, "\n"))
}

func (r *Renderer) renderHistoryTextBlock(block uimsg.HistoryBlock, width int) string {
	parts := make([]string, 0, 2)
	if block.Title != "" {
		parts = append(parts, r.styles.Command.SectionTitle.Render(block.Title))
	}

	parts = append(parts, wordwrap.String(block.Text, width))

	return bubbleContentStyle(block, r.styles.CommandBubble.Content).Render(strings.Join(parts, "\n"))
}

func (r *Renderer) renderHistoryMarkdownBlock(block uimsg.HistoryBlock, _ int) string {
	parts := make([]string, 0, 2)
	if block.Title != "" {
		parts = append(parts, r.styles.Command.SectionTitle.Render(block.Title))
	}

	rendered := block.Text
	if mdContent, err := r.mdRenderer.Render(block.Text); err == nil {
		rendered = mdContent
	}

	parts = append(parts, rendered)

	return bubbleContentStyle(block, r.styles.CommandBubble.Content).Render(strings.Join(parts, "\n"))
}

func bubbleContentStyle(block uimsg.HistoryBlock, style lipgloss.Style) lipgloss.Style {
	if block.Title == "" {
		return style
	}

	return style.UnsetMarginTop()
}
