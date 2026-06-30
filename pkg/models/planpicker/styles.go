package planpicker

import "github.com/charmbracelet/lipgloss"

type Styles struct {
	Container lipgloss.Style
	Title     lipgloss.Style
	Help      lipgloss.Style
	Empty     lipgloss.Style
	Item      lipgloss.Style
	Selected  lipgloss.Style
	PlanName  lipgloss.Style
	LogPath   lipgloss.Style
	Cursor    lipgloss.Style
	Count     lipgloss.Style
}

func InitStyles() Styles {
	var (
		black      = lipgloss.Color("#000000")
		panel      = lipgloss.Color("#11161d")
		border     = lipgloss.Color("#3f4856")
		orange     = lipgloss.Color("#ff9f1c")
		gold       = lipgloss.Color("#ffd166")
		blue       = lipgloss.Color("#8fb7ff")
		cyan       = lipgloss.Color("#8bd2e6")
		white      = lipgloss.Color("#ffffff")
		text       = lipgloss.Color("#d7e0ea")
		muted      = lipgloss.Color("#7d8796")
		selectedBG = lipgloss.Color("#321b05")
	)

	return Styles{
		Container: lipgloss.NewStyle().
			Background(panel).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(border).
			BorderBackground(black).
			Padding(1, 2),
		Title: lipgloss.NewStyle().
			Foreground(blue).
			Background(panel).
			Bold(true),
		Help: lipgloss.NewStyle().
			Foreground(muted).
			Background(panel),
		Empty: lipgloss.NewStyle().
			Foreground(muted).
			Background(panel).
			Italic(true),
		Item: lipgloss.NewStyle().
			Foreground(text).
			Background(panel),
		Selected: lipgloss.NewStyle().
			Foreground(white).
			Background(selectedBG).
			Border(lipgloss.ThickBorder(), false, false, false, true).
			BorderForeground(orange).
			Padding(0, 1).
			Bold(true),
		PlanName: lipgloss.NewStyle().
			Foreground(text).
			Background(panel),
		LogPath: lipgloss.NewStyle().
			Foreground(muted).
			Background(panel),
		Cursor: lipgloss.NewStyle().
			Foreground(gold).
			Background(selectedBG).
			Bold(true),
		Count: lipgloss.NewStyle().
			Foreground(cyan).
			Background(panel),
	}
}

func (s Styles) SizedContainer(width int) lipgloss.Style {
	if width <= 0 {
		return s.Container
	}

	return s.Container.Width(max(20, width-2))
}

func (s Styles) ItemStyle(selected bool) lipgloss.Style {
	if selected {
		return s.Selected
	}

	return s.Item
}

func (s Styles) PlanNameStyle(selected bool) lipgloss.Style {
	if selected {
		return s.PlanName.Foreground(lipgloss.Color("#fff2cc")).Background(lipgloss.Color("#321b05")).Bold(true)
	}

	return s.PlanName
}

func (s Styles) LogPathStyle(selected bool) lipgloss.Style {
	if selected {
		return s.LogPath.Foreground(lipgloss.Color("#d6b06a")).Background(lipgloss.Color("#321b05"))
	}

	return s.LogPath
}
