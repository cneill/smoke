package statusline

import "github.com/charmbracelet/lipgloss"

type Styles struct {
	BorderFocused lipgloss.Style
	BorderBlurred lipgloss.Style

	UsageFocused lipgloss.Style
	UsageBlurred lipgloss.Style

	// Secondary is a dimmer variant of Usage for less important text like the provider name and labels.
	SecondaryFocused lipgloss.Style
	SecondaryBlurred lipgloss.Style
}

func InitStyles() Styles {
	var (
		black     = lipgloss.Color("#000000")
		darkgray  = lipgloss.Color("#444444")
		midgray   = lipgloss.Color("#666666")
		lightgray = lipgloss.Color("#bbbbbb")
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
			Foreground(lightgray).
			Bold(true),
		UsageBlurred: usageBase.
			Foreground(lightgray),

		SecondaryFocused: usageBase.
			Foreground(midgray).
			Bold(true),
		SecondaryBlurred: usageBase.
			Foreground(darkgray),
	}
}

func (s Styles) GetVariant(variant StyleVariant) Style {
	focused := Style{
		Border:    s.BorderFocused,
		Usage:     s.UsageFocused,
		Secondary: s.SecondaryFocused,
	}

	switch variant {
	case Focused:
		return focused
	case Blurred:
		return Style{
			Border:    s.BorderBlurred,
			Usage:     s.UsageBlurred,
			Secondary: s.SecondaryBlurred,
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
	Border    lipgloss.Style
	Usage     lipgloss.Style
	Secondary lipgloss.Style
}
