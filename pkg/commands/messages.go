package commands

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/cneill/smoke/pkg/llms"
)

type Message interface {
	isCommandMessage()
}

// PromptCommandMessage contains the details of a prompt command sent by the user. It is not validated in advance.
type PromptCommandMessage struct {
	Command string
	Args    []string
}

func (p PromptCommandMessage) isCommandMessage() {}

type ErrorMessage struct {
	Err error
}

func (e ErrorMessage) isCommandMessage() {}

// HistoryUpdateMessage is used to send a message back from the prompt command handler to update the history UI.
type HistoryUpdateMessage struct {
	PromptCommand PromptCommandMessage
	Message       string
}

func (h HistoryUpdateMessage) isCommandMessage() {}

// SessionUpdateMessage is used by a prompt command to update the session used by Smoke
type SessionUpdateMessage struct {
	PromptCommand PromptCommandMessage
	Session       *llms.Session
	ResetHistory  bool
	Message       string
}

func (s SessionUpdateMessage) isCommandMessage() {}

// TODO: have a single mode message that returns a session update message in a tea.Batch

// PlanningModeMessage signals to Smoke to either enable or disable planning mode.
type PlanningModeMessage struct {
	PromptCommand PromptCommandMessage
	Enabled       bool
	Message       string
	Session       *llms.Session
}

func (p PlanningModeMessage) isCommandMessage() {}

// ReviewModeMessage signals to Smoke to either enable or disable review mode.
type ReviewModeMessage struct {
	PromptCommand PromptCommandMessage
	Enabled       bool
	Message       string
	Session       *llms.Session
}

func (r ReviewModeMessage) isCommandMessage() {}

// EditRequestMessage asks the UI to open a given file path in an editor, suspending the TUI.
type EditRequestMessage struct {
	PromptCommand PromptCommandMessage
	Target        string
	Path          string
	Editor        string
	Description   string
}

func (e EditRequestMessage) isCommandMessage() {}

// EditResultMessage reports the result of trying to open the editor for a given path.
type EditResultMessage struct {
	EditRequestMessage

	Err error
}

func (e EditResultMessage) isCommandMessage() {}

// SendSession is used to send a session to an LLM and get the response asynchronously.
type SendSessionMessage struct {
	PromptCommand    PromptCommandMessage
	OriginalMessages []*llms.Message
	Session          *llms.Session
}

func (s SendSessionMessage) isCommandMessage() {}

func MsgToCmd(msg any) tea.Cmd {
	return func() tea.Msg {
		return msg
	}
}
