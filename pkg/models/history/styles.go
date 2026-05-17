package history

import "github.com/charmbracelet/lipgloss"

const defaultBubbleWidth = 64

type Styles struct {
	BaseBubble BubbleStyle

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

type BubbleStyle struct {
	Container   lipgloss.Style
	Title       lipgloss.Style
	Subtitle    lipgloss.Style
	Content     lipgloss.Style
	UseMarkdown bool
}

func InitStyles() Styles { //nolint:funlen
	base := BubbleStyle{
		Container: lipgloss.NewStyle().
			Width(defaultBubbleWidth).
			Padding(0, 2).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#3f4856")).
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
		UseMarkdown: false,
	}

	return Styles{
		BaseBubble: base,
		UserBubble: BubbleStyle{
			Container:   base.Container,
			Title:       base.Title.Foreground(lipgloss.Color("#0087ff")),
			Subtitle:    base.Subtitle,
			Content:     base.Content,
			UseMarkdown: true,
		},
		AssistantBubble: BubbleStyle{
			Container:   base.Container,
			Title:       base.Title.Foreground(lipgloss.Color("#00af00")),
			Subtitle:    base.Subtitle,
			Content:     base.Content,
			UseMarkdown: true,
		},
		ToolBubble: BubbleStyle{
			Container:   base.Container,
			Title:       base.Title.Foreground(lipgloss.Color("#00afaf")),
			Subtitle:    base.Subtitle,
			Content:     base.Content,
			UseMarkdown: false,
		},
		SystemBubble: BubbleStyle{
			Container:   base.Container,
			Title:       base.Title.Foreground(lipgloss.Color("#af00af")),
			Subtitle:    base.Subtitle,
			Content:     base.Content,
			UseMarkdown: true,
		},
		CommandBubble: BubbleStyle{
			Container:   base.Container,
			Title:       base.Title.Foreground(lipgloss.Color("#dd9911")),
			Subtitle:    base.Subtitle,
			Content:     base.Content,
			UseMarkdown: true,
		},
		ErrorBubble: BubbleStyle{
			Container:   base.Container,
			Title:       base.Title.Foreground(lipgloss.Color("#af0000")),
			Subtitle:    base.Subtitle,
			Content:     base.Content,
			UseMarkdown: false,
		},
		UnknownBubble: BubbleStyle{
			Container:   base.Container,
			Title:       base.Title.Foreground(lipgloss.Color("#999999")),
			Subtitle:    base.Subtitle,
			Content:     base.Content,
			UseMarkdown: false,
		},
		SessionBubble: BubbleStyle{
			Container:   base.Container,
			Title:       base.Title.Foreground(lipgloss.Color("#ffffff")),
			Subtitle:    base.Subtitle,
			Content:     base.Content,
			UseMarkdown: false,
		},
		ElicitBubble: BubbleStyle{
			Container:   base.Container,
			Title:       base.Title.Foreground(lipgloss.Color("#afaf00")),
			Subtitle:    base.Subtitle,
			Content:     base.Content,
			UseMarkdown: false,
		},
		ElicitCanceledBubble: BubbleStyle{
			Container:   base.Container,
			Title:       base.Title.Foreground(lipgloss.Color("#ff0000")),
			Subtitle:    base.Subtitle,
			Content:     base.Content,
			UseMarkdown: false,
		},
	}
}
