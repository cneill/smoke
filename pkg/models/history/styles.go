package history

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/reflow/wordwrap"
)

const defaultBubbleWidth = 64

type Styles struct {
	BaseBubble BubbleStyle
	Command    CommandContentStyles

	// Session message styles
	UserBubble      BubbleStyle
	AssistantBubble BubbleStyle
	ToolBubble      BubbleStyle
	SystemBubble    BubbleStyle

	// Other message styles
	CommandBubble        BubbleStyle
	ErrorBubble          BubbleStyle
	UnknownBubble        BubbleStyle
	SessionBubble        BubbleStyle
	ElicitBubble         BubbleStyle
	ElicitCanceledBubble BubbleStyle
}

type CommandContentStyles struct {
	SectionTitle lipgloss.Style
	FieldLabel   lipgloss.Style
	FieldValue   lipgloss.Style
}

type BubbleStyle struct {
	Container lipgloss.Style
	Title     lipgloss.Style
	Subtitle  lipgloss.Style
	Content   lipgloss.Style
}

func InitStyles(width int) Styles { //nolint:funlen
	base := BubbleStyle{
		Container: lipgloss.NewStyle().
			Width(width-2).
			Padding(0, 2).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#3f4856")).
			BorderBackground(lipgloss.Color("#11161d")).
			Background(lipgloss.Color("#11161d")),
		Title: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#8fb7ff")).
			Background(lipgloss.Color("#11161d")).
			Bold(true),
		Subtitle: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#7d8796")).
			Background(lipgloss.Color("#11161d")).
			Italic(true),
		Content: lipgloss.NewStyle().
			MarginTop(1),
	}

	commandContent := CommandContentStyles{
		SectionTitle: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#8fb7ff")).
			Bold(true).
			Border(lipgloss.NormalBorder()).
			BorderForeground(lipgloss.Color("#8fb7ff")).
			MarginBottom(1),
		FieldLabel: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#dd9911")).
			Bold(true),
		FieldValue: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#d7e0ea")),
	}

	return Styles{
		BaseBubble: base,
		Command:    commandContent,
		UserBubble: BubbleStyle{
			Container: base.Container,
			Title:     base.Title.Foreground(lipgloss.Color("#0087ff")),
			Subtitle:  base.Subtitle,
			Content:   base.Content,
		},
		AssistantBubble: BubbleStyle{
			Container: base.Container,
			Title:     base.Title.Foreground(lipgloss.Color("#00af00")),
			Subtitle:  base.Subtitle,
			Content:   base.Content,
		},
		ToolBubble: BubbleStyle{
			Container: base.Container,
			Title:     base.Title.Foreground(lipgloss.Color("#00afaf")),
			Subtitle:  base.Subtitle,
			Content:   base.Content,
		},
		SystemBubble: BubbleStyle{
			Container: base.Container,
			Title:     base.Title.Foreground(lipgloss.Color("#af00af")),
			Subtitle:  base.Subtitle,
			Content:   base.Content,
		},
		CommandBubble: BubbleStyle{
			Container: base.Container,
			Title:     base.Title.Foreground(lipgloss.Color("#dd9911")),
			Subtitle:  base.Subtitle,
			Content:   base.Content,
		},
		ErrorBubble: BubbleStyle{
			Container: base.Container,
			Title:     base.Title.Foreground(lipgloss.Color("#af0000")),
			Subtitle:  base.Subtitle,
			Content:   base.Content,
		},
		UnknownBubble: BubbleStyle{
			Container: base.Container,
			Title:     base.Title.Foreground(lipgloss.Color("#999999")),
			Subtitle:  base.Subtitle,
			Content:   base.Content,
		},
		SessionBubble: BubbleStyle{
			Container: base.Container,
			Title:     base.Title.Foreground(lipgloss.Color("#ffffff")),
			Subtitle:  base.Subtitle,
			Content:   base.Content,
		},
		ElicitBubble: BubbleStyle{
			Container: base.Container,
			Title:     base.Title.Foreground(lipgloss.Color("#afaf00")),
			Subtitle:  base.Subtitle,
			Content:   base.Content,
		},
		ElicitCanceledBubble: BubbleStyle{
			Container: base.Container,
			Title:     base.Title.Foreground(lipgloss.Color("#ff0000")),
			Subtitle:  base.Subtitle,
			Content:   base.Content,
		},
	}
}

func (s CommandContentStyles) RenderField(label, value string, width int) string {
	prefix := s.FieldLabel.Render(label + ":")
	if strings.TrimSpace(value) == "" {
		return prefix
	}

	availableWidth := width - lipgloss.Width(prefix) - 1
	if availableWidth < 1 {
		availableWidth = width
	}

	wrappedValue := wordwrap.String(value, availableWidth)

	valueLines := strings.Split(wrappedValue, "\n")
	for i, line := range valueLines {
		valueLines[i] = s.FieldValue.Render(line)
	}

	if len(valueLines) == 1 {
		return prefix + " " + valueLines[0]
	}

	indent := strings.Repeat(" ", lipgloss.Width(prefix)+1)

	return prefix + " " + valueLines[0] + "\n" + indent + strings.Join(valueLines[1:], "\n"+indent)
}
