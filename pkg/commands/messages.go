package commands

import (
	"github.com/cneill/smoke/pkg/llms"
)

// Message is a simple interface to identify all messages that originate from prompt commands.
type Message interface {
	isCommandMessage()
}

// MessageType is a helper type that implements Message so that individual message structs in other packages can
// be identified as commands.Message.
type MessageType struct{}

func (m MessageType) isCommandMessage() {}

// PromptMessage contains the details of a prompt command sent by the user. It is not validated in advance.
type PromptMessage struct {
	MessageType

	Command string
	Args    []string
}

// HistoryUpdateMessage is used to send a simple message back from the prompt command handler to update the history UI.
type HistoryUpdateMessage struct {
	MessageType

	PromptMessage PromptMessage
	Message       string
}

// SessionUpdateMessage is used by a prompt command to update the session used by Smoke
type SessionUpdateMessage struct {
	MessageType

	PromptMessage PromptMessage
	Session       *llms.Session
	ResetHistory  bool
	Message       string
}

// SendSessionMessage is used to send a session to an LLM and get the response asynchronously.
type SendSessionMessage struct {
	MessageType

	PromptMessage    PromptMessage
	PromptCommand    PromptMessage
	OriginalMessages []*llms.Message
	Session          *llms.Session
}

type ErrorMessage struct {
	MessageType

	Err error
}

type HelpMessage struct {
	MessageType

	Text string
}
