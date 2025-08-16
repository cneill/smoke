package commands

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/cneill/smoke/pkg/llms"
)

// PromptCommandMessage contains the details of a prompt command sent by the user. It is not validated in advance.
type PromptCommandMessage struct {
	Command string
	Args    []string
}

type ErrorMessage struct {
	Err error
}

// HistoryUpdateMessage is used to send a message back from the prompt command handler to update the history UI.
type HistoryUpdateMessage struct {
	PromptCommand PromptCommandMessage
	Message       string
}

// Cmd returns a tea.Cmd to update the history.
func (h HistoryUpdateMessage) Cmd() tea.Cmd {
	return func() tea.Msg {
		return h
	}
}

// SessionUpdateMessage is used by a prompt command to update the session used by Smoke
type SessionUpdateMessage struct {
	PromptCommand PromptCommandMessage
	Session       *llms.Session
	Message       string
}

// Cmd returns a tea.Cmd to update the session.
func (s SessionUpdateMessage) Cmd() tea.Cmd {
	return func() tea.Msg {
		return s
	}
}

// PlanningModeMessage signals to Smoke to either enable or disable planning mode.
type PlanningModeMessage struct {
	PromptCommand PromptCommandMessage
	Enabled       bool
	Message       string
}

// Cmd returns a tea.Cmd to enable or disable planning mode.
func (p PlanningModeMessage) Cmd() tea.Cmd {
	return func() tea.Msg {
		return p
	}
}
