package commands

import tea "github.com/charmbracelet/bubbletea"

type PromptCommandMessage struct {
	Command string
	Args    []string
}

type ErrorMessage struct {
	Err error
}

type HistoryUpdateMessage struct {
	PromptCommand PromptCommandMessage
	Message       string
}

func (h HistoryUpdateMessage) Cmd() tea.Cmd {
	return func() tea.Msg {
		return h
	}
}
