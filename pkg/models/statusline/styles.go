package statusline

import "github.com/charmbracelet/lipgloss"

type Styles struct {
	BorderFocused lipgloss.Style
	BorderBlurred lipgloss.Style

	SuggestionFocused lipgloss.Style
	SuggestionBlurred lipgloss.Style

	UsageFocused lipgloss.Style
	UsageBlurred lipgloss.Style
}

func InitStyles() Styles {
	var (
		black       = lipgloss.Color("#000000")
		brightgreen = lipgloss.Color("#00ff00")
		darkgray    = lipgloss.Color("#333333")
		lightgray   = lipgloss.Color("#aaaaaa")
		orange      = lipgloss.Color("#cc4400")
		white       = lipgloss.Color("#ffffff")
	)

	borderBase := lipgloss.NewStyle().
		Background(black).
		Align(lipgloss.Left)

	suggestionBase := lipgloss.NewStyle().
		Foreground(white).
		Align(lipgloss.Left)

	usageBase := lipgloss.NewStyle().
		Background(black).
		Align(lipgloss.Left)

	return Styles{
		BorderFocused: borderBase.
			Foreground(orange),
		BorderBlurred: borderBase.
			Foreground(darkgray),

		SuggestionFocused: suggestionBase.
			Background(orange).
			Bold(true),
		SuggestionBlurred: suggestionBase.
			Background(lightgray),

		UsageFocused: usageBase.
			Foreground(brightgreen).
			Bold(true),
		UsageBlurred: usageBase.
			Foreground(lightgray),
	}
}

func (s Styles) GetVariant(variant StyleVariant) Style {
	focused := Style{
		Border:     s.BorderFocused,
		Suggestion: s.SuggestionFocused,
		Usage:      s.UsageFocused,
	}

	switch variant {
	case Focused:
		return focused
	case Blurred:
		return Style{
			Border:     s.BorderBlurred,
			Suggestion: s.SuggestionBlurred,
			Usage:      s.UsageBlurred,
		}
	}

	return focused
}

type StyleVariant int

const (
	Focused StyleVariant = iota
	Blurred
)

type Style struct {
	Border     lipgloss.Style
	Suggestion lipgloss.Style
	Usage      lipgloss.Style
}
