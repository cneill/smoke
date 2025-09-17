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
	ResetHistory  bool
	Message       string
}

// Cmd returns a tea.Cmd to update the session.
func (s SessionUpdateMessage) Cmd() tea.Cmd {
	return func() tea.Msg {
		return s
	}
}

// TODO: have a single mode message that returns a session update message in a tea.Batch

// PlanningModeMessage signals to Smoke to either enable or disable planning mode.
type PlanningModeMessage struct {
	PromptCommand PromptCommandMessage
	Enabled       bool
	Message       string
	Session       *llms.Session
}

// Cmd returns a tea.Cmd to enable or disable planning mode.
func (p PlanningModeMessage) Cmd() tea.Cmd {
	return func() tea.Msg {
		return p
	}
}

// ReviewModeMessage signals to Smoke to either enable or disable review mode.
type ReviewModeMessage struct {
	PromptCommand PromptCommandMessage
	Enabled       bool
	Message       string
	Session       *llms.Session
}

// Cmd returns a tea.Cmd to enable or disable review mode.
func (r ReviewModeMessage) Cmd() tea.Cmd {
	return func() tea.Msg {
		return r
	}
}

// EditRequestMessage asks the UI to open a given file path in an editor, suspending the TUI.
type EditRequestMessage struct {
	PromptCommand PromptCommandMessage
	Target        string
	Path          string
	Editor        string
	Description   string
}

// Cmd returns a tea.Cmd to send this message.
func (e EditRequestMessage) Cmd() tea.Cmd {
	return func() tea.Msg { return e }
}

// EditResultMessage reports the result of trying to open the editor for a given path.
type EditResultMessage struct {
	EditRequestMessage

	Err error
}

// Cmd returns a tea.Cmd to send this message.
func (e EditResultMessage) Cmd() tea.Cmd {
	return func() tea.Msg { return e }
}

// SendSession is used to send a session to an LLM and get the response asynchronously.
type SendSessionMessage struct {
	PromptCommand    PromptCommandMessage
	OriginalMessages []*llms.Message
	Session          *llms.Session
}

func (s SendSessionMessage) Cmd() tea.Cmd {
	return func() tea.Msg {
		return s
	}
}
