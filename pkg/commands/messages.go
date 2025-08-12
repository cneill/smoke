package commands

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/cneill/smoke/pkg/llms"
)

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

type SessionUpdateMessage struct {
	PromptCommand PromptCommandMessage
	Session       *llms.Session
	Message       string
}

func (s SessionUpdateMessage) Cmd() tea.Cmd {
	return func() tea.Msg {
		return s
	}
}
