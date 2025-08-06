package models

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/cneill/smoke/pkg/models/banner"
	"github.com/cneill/smoke/pkg/models/history"
	"github.com/cneill/smoke/pkg/models/input"
	"golang.org/x/term"
)

const gap = "\n"

type Model struct {
	banner  *banner.Model
	history *history.Model
	input   *input.Model
}

func New() (*Model, error) {
	width, height, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil {
		return nil, fmt.Errorf("failed to get terminal size: %w", err)
	}

	bannerModel := banner.New()

	historyModel, err := history.New(width, height-2)
	if err != nil {
		return nil, fmt.Errorf("failed to set up history view: %w", err)
	}

	inputModel, err := input.New(width, 2)
	if err != nil {
		return nil, fmt.Errorf("failed to set up input view: %w", err)
	}

	model := &Model{
		banner:  bannerModel,
		history: historyModel,
		input:   inputModel,
	}

	return model, nil
}

func (m *Model) Init() tea.Cmd {
	return tea.Batch(m.history.Init(), m.input.Init())
}

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	commands := []tea.Cmd{}

	switch msg := msg.(type) {
	case tea.WindowSizeMsg, input.ResizeMessage:
		m.resize(msg)
	case tea.KeyMsg:
		switch msg.Type { //nolint:exhaustive
		case tea.KeyCtrlC:
			return m, tea.Quit
		default:
			inputModel, inputCmd := m.input.Update(msg)
			m.input = inputModel

			commands = append(commands, inputCmd) // TODO: wrap?

			historyModel, historyCmd := m.history.Update(msg)
			m.history = historyModel

			commands = append(commands, historyCmd) // TODO: wrap/
		}
	case history.Message:
		historyModel, historyCmd := m.history.Update(msg)
		m.history = historyModel

		commands = append(commands, historyCmd)
	case input.Message:
		newInput, inputCmd := m.input.Update(msg)
		m.input = newInput

		commands = append(commands, inputCmd)
	}

	return m, tea.Batch(commands...)
}

func (m *Model) resize(msg tea.Msg) {
	lineHeight := m.input.LineHeight()

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.history.Resize(msg.Width, msg.Height-lineHeight)
		m.input.Resize(msg.Width, lineHeight)
	case input.ResizeMessage:
		delta := lineHeight - m.input.GetHeight() // how many lines did we resize by
		width := m.history.GetWidth()
		m.history.Resize(width, m.history.GetHeight()-delta)
		m.input.Resize(width, lineHeight)
	}
}

func (m *Model) View() string {
	return fmt.Sprintf("%s%s%s", m.history.View(), gap, m.input.View())
}
