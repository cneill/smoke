package history

import (
	"fmt"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/cneill/smoke/pkg/models/banner"
)

type Model struct {
	banner     *banner.Model
	viewport   viewport.Model
	mdRenderer *glamour.TermRenderer

	haveContent bool // TODO: can this be eliminated with something checking the viewport for message content?
}

func New(width, height int) (*Model, error) {
	banner := banner.New()
	viewport := viewport.New(width, height)
	viewport.SetContent(banner.View())

	mdRenderer, err := getGlamourRenderer(width)
	if err != nil {
		return nil, err
	}

	model := &Model{
		banner:     banner,
		viewport:   viewport,
		mdRenderer: mdRenderer,

		haveContent: false,
	}

	return model, nil
}

func getGlamourRenderer(width int) (*glamour.TermRenderer, error) {
	renderer, err := glamour.NewTermRenderer(
		glamour.WithWordWrap(width-4),
		glamour.WithEmoji(),
		glamour.WithAutoStyle(),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to set up markdown renderer: %w", err)
	}

	return renderer, nil
}

func (m *Model) Init() tea.Cmd {
	return nil
}

func (m *Model) Update(msg tea.Msg) (*Model, tea.Cmd) {
	switch msg := msg.(type) { //nolint:gocritic
	case Message:
		switch msg.Underlying.(type) { //nolint:gocritic
		case ContentUpdate:
			// TODO: render bubbles / etc
			m.haveContent = true
		}
	}

	return m, nil
}

func (m *Model) View() string {
	return m.viewport.View()
}

func (m *Model) Resize(width, height int) {
	m.viewport.Width = width
	m.viewport.Height = height
}

func (m *Model) GetWidth() int {
	return m.viewport.Width
}

func (m *Model) GetHeight() int {
	return m.viewport.Height
}

type Message struct {
	Underlying tea.Msg
}

type ContentUpdate struct{}
