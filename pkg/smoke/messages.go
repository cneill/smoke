package smoke

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/cneill/smoke/pkg/commands"
	"github.com/cneill/smoke/pkg/llms"
)

type AssistantResponseMessage struct {
	Message *llms.Message
	Err     error
}

// TODO: need this?
type AssistantUpdatedStreamMessage struct {
	Message *llms.Message
}

type ToolCallResponseMessage struct {
	Messages []*llms.Message
	Err      error
}

// TODO: better name
type SendCommandMessageResponseMessage struct {
	OriginalMessage commands.SendSessionMessage
	Session         *llms.Session
	Err             error
}

func MsgToCmd(msg tea.Msg) tea.Cmd {
	return func() tea.Msg {
		return msg
	}
}
