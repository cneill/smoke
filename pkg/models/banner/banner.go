package banner

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const bannerText = `
 ░▒▓███████▓▒░▒▓██████████████▓▒░ ░▒▓██████▓▒░░▒▓█▓▒░░▒▓█▓▒░▒▓████████▓▒░ 
░▒▓█▓▒░      ░▒▓█▓▒░░▒▓█▓▒░░▒▓█▓▒░▒▓█▓▒░░▒▓█▓▒░▒▓█▓▒░░▒▓█▓▒░▒▓█▓▒░        
░▒▓█▓▒░      ░▒▓█▓▒░░▒▓█▓▒░░▒▓█▓▒░▒▓█▓▒░░▒▓█▓▒░▒▓█▓▒░░▒▓█▓▒░▒▓█▓▒░        
 ░▒▓██████▓▒░░▒▓█▓▒░░▒▓█▓▒░░▒▓█▓▒░▒▓█▓▒░░▒▓█▓▒░▒▓███████▓▒░░▒▓██████▓▒░   
       ░▒▓█▓▒░▒▓█▓▒░░▒▓█▓▒░░▒▓█▓▒░▒▓█▓▒░░▒▓█▓▒░▒▓█▓▒░░▒▓█▓▒░▒▓█▓▒░        
       ░▒▓█▓▒░▒▓█▓▒░░▒▓█▓▒░░▒▓█▓▒░▒▓█▓▒░░▒▓█▓▒░▒▓█▓▒░░▒▓█▓▒░▒▓█▓▒░        
░▒▓███████▓▒░░▒▓█▓▒░░▒▓█▓▒░░▒▓█▓▒░░▒▓██████▓▒░░▒▓█▓▒░░▒▓█▓▒░▒▓████████▓▒░ 
`

func interpolate(start, end, step, totalSteps int) int {
	return start + ((end-start)*step)/totalSteps
}

type Model struct {
	StartColor lipgloss.Color
	EndColor   lipgloss.Color

	rendered string
}

func New() *Model {
	return &Model{
		StartColor: lipgloss.Color("#ffffff"),
		EndColor:   lipgloss.Color("#111111"),
	}
}

func (m *Model) Init() tea.Cmd {
	return nil
}

func (m *Model) Update(_ tea.Msg) (tea.Model, tea.Cmd) {
	return m, nil
}

func (m *Model) View() string {
	if m.rendered != "" {
		return m.rendered
	}

	builder := &strings.Builder{}
	lines := strings.Split(bannerText, "\n")
	numLines := len(lines)

	for lineNum, line := range lines {
		r1, g1, b1, _ := m.StartColor.RGBA()
		r2, g2, b2, _ := m.EndColor.RGBA()

		r := interpolate(int(r1>>8), int(r2>>8), lineNum, numLines-1)
		g := interpolate(int(g1>>8), int(g2>>8), lineNum, numLines-1)
		b := interpolate(int(b1>>8), int(b2>>8), lineNum, numLines-1)

		newColor := lipgloss.Color(fmt.Sprintf("#%02x%02x%02x", r, g, b))
		style := lipgloss.NewStyle().Foreground(newColor)
		fmt.Fprintln(builder, style.Render(line))
	}

	m.rendered = builder.String()

	return m.rendered
}
