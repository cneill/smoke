package statusline

import "github.com/charmbracelet/lipgloss"

type Styles struct {
	BorderFocused lipgloss.Style
	BorderBlurred lipgloss.Style

	UsageFocused lipgloss.Style
	UsageBlurred lipgloss.Style
}

func InitStyles() Styles {
	var (
		black     = lipgloss.Color("#000000")
		usageText = lipgloss.Color("#8bd2e6")
		darkgray  = lipgloss.Color("#333333")
		lightgray = lipgloss.Color("#aaaaaa")
		orange    = lipgloss.Color("#cc4400")
	)

	borderBase := lipgloss.NewStyle().
		Background(black).
		Align(lipgloss.Left)

	usageBase := lipgloss.NewStyle().
		Background(black).
		Align(lipgloss.Left)

	return Styles{
		BorderFocused: borderBase.
			Foreground(orange),
		BorderBlurred: borderBase.
			Foreground(darkgray),

		UsageFocused: usageBase.
			Foreground(usageText).
			Bold(true),
		UsageBlurred: usageBase.
			Foreground(lightgray),
	}
}

func (s Styles) GetVariant(variant StyleVariant) Style {
	focused := Style{
		Border: s.BorderFocused,
		Usage:  s.UsageFocused,
	}

	switch variant {
	case Focused:
		return focused
	case Blurred:
		return Style{
			Border: s.BorderBlurred,
			Usage:  s.UsageBlurred,
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
	Border lipgloss.Style
	Usage  lipgloss.Style
}
