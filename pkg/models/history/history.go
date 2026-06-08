// Package history contains a Bubbletea model for holding onto LLM chat history and other items like errors, tool calls,
// prompt commands, etc.
package history

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
)

type Opts struct {
	Width       int
	Height      int
	InitContent string
}

func (o *Opts) OK() error {
	switch {
	case o.Width <= 0:
		return fmt.Errorf("width must be >0")
	case o.Height <= 0:
		return fmt.Errorf("height must be >0")
	}

	return nil
}

type Model struct {
	viewport viewport.Model
	renderer *Renderer

	initContent string
	log         *Log

	pendingG  bool
	lastGTime time.Time
}

func New(opts *Opts) (*Model, error) {
	if err := opts.OK(); err != nil {
		return nil, fmt.Errorf("options error: %w", err)
	}

	renderer, err := NewRenderer(opts.Width)
	if err != nil {
		return nil, err
	}

	model := &Model{
		viewport: getViewport(opts),
		renderer: renderer,

		initContent: opts.InitContent,
		log:         NewLog(),
	}

	return model, nil
}

func getViewport(opts *Opts) viewport.Model {
	newViewport := viewport.New(opts.Width, opts.Height)
	newViewport.SetContent(opts.InitContent)

	keyMap := viewport.DefaultKeyMap()
	keyMap.PageUp = key.NewBinding(
		key.WithKeys("pgup", "ctrl+b"),
		key.WithHelp("pgup/ctrl+b", "page up"),
	)
	keyMap.PageDown = key.NewBinding(
		key.WithKeys("pgdown", "ctrl+f"),
		key.WithHelp("pgdown/ctrl+f", "page down"),
	)

	newViewport.KeyMap = keyMap

	return newViewport
}

func getGlamourRenderer(width int) (*glamour.TermRenderer, error) {
	renderer, err := glamour.NewTermRenderer(
		glamour.WithWordWrap(width),
		glamour.WithEmoji(),
		glamour.WithAutoStyle(),
		glamour.WithPreservedNewLines(),
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
	wasAtBottom := m.viewport.AtBottom()

	switch msg := msg.(type) {
	case ContentUpdate:
		m.log.AddMessage(msg.Message)
		m.viewport.SetContent(m.renderLogContent())
		// only scroll down if we were already at the bottom before updating the history
		if wasAtBottom {
			m.viewport.GotoBottom()
		}
	case ContentRefresh:
		m.log.RefreshLog(msg.Log)

		if logContent := m.renderLogContent(); strings.TrimSpace(logContent) != "" {
			m.viewport.SetContent(logContent)
		} else {
			m.viewport.SetContent(m.initContent)
		}

		if wasAtBottom {
			m.viewport.GotoBottom()
		}
	case tea.KeyMsg:
		return m, m.handleKeyMsg(msg)

	default:
		var cmd tea.Cmd

		m.viewport, cmd = m.viewport.Update(msg)

		return m, cmd
	}

	return m, nil
}

func (m *Model) View() string {
	return m.viewport.View()
}

func (m *Model) Resize(width, height int) {
	m.viewport.Width = width
	m.viewport.Height = height

	m.renderer.Resize(width)
}

func (m *Model) GetWidth() int {
	return m.viewport.Width
}

func (m *Model) GetHeight() int {
	return m.viewport.Height
}

func (m *Model) handleKeyMsg(msg tea.KeyMsg) tea.Cmd {
	var cmd tea.Cmd

	// Handle VIM-like scrolling commands
	switch msg.String() {
	case "G":
		m.viewport.GotoBottom()
		m.pendingG = false
	case "g":
		if m.pendingG && time.Since(m.lastGTime) <= time.Second {
			m.viewport.GotoTop()
			m.pendingG = false
		} else {
			m.lastGTime = time.Now()
			m.pendingG = true
		}
	default:
		m.pendingG = false
		m.viewport, cmd = m.viewport.Update(msg)
	}

	return cmd
}

func (m *Model) renderLogContent() string {
	builder := &strings.Builder{}
	styles := m.renderer.Styles()

	for _, item := range m.log.Messages() {
		bubble := BubbleForHistoryItem(item, styles)
		// Currently used for epmty Assistant messages
		// TODO: make this less convoluted
		if bubble.IsEmpty() {
			continue
		}

		fmt.Fprint(builder, m.renderer.RenderBubble(bubble))
		builder.WriteRune('\n')
	}

	return builder.String()
}
