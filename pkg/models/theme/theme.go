package theme

import "github.com/charmbracelet/lipgloss"

type Theme struct {
	History HistoryStyle
	Input   InputStyle
}

func DefaultTheme() *Theme {
	return &Theme{
		History: HistoryStyle{
			Primary: lipgloss.NewStyle().
				Border(lipgloss.NormalBorder()).
				BorderForeground(lipgloss.Color("#222222")).
				Padding(2),
		},
		Input: InputStyle{
			Focused: lipgloss.NewStyle().
				Background(lipgloss.Color("#000000")).
				Foreground(lipgloss.Color("#eeeeee")).
				Padding(2, 2, 2, 2),
			Blurred: lipgloss.NewStyle().
				Background(lipgloss.Color("#333333")).
				Foreground(lipgloss.Color("#111111")).
				Padding(2, 2, 2, 2),
		},
	}
}

type HistoryStyle struct {
	Primary lipgloss.Style
}

type InputStyle struct {
	Focused lipgloss.Style
	Blurred lipgloss.Style
}
