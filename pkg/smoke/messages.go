package smoke

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/cneill/smoke/internal/uimsg"
	"github.com/cneill/smoke/pkg/commands"
	"github.com/cneill/smoke/pkg/llmctx/modes"
	"github.com/cneill/smoke/pkg/llms"
)

type Message interface {
	isSmokeMessage()
}

type AssistantResponseMessage struct {
	Message *llms.Message
	Err     *uimsg.Error
}

func (a AssistantResponseMessage) isSmokeMessage() {}

type AssistantUpdatedStreamMessage struct {
	Message *llms.Message
}

func (a AssistantUpdatedStreamMessage) isSmokeMessage() {}

type UsageUpdateMessage struct {
	InputTokens  int64
	OutputTokens int64
}

func (u UsageUpdateMessage) isSmokeMessage() {}

type ToolCallResponseMessage struct {
	Messages []*llms.Message
	Err      *uimsg.Error
}

func (t ToolCallResponseMessage) isSmokeMessage() {}

// TODO: better name
type SendCommandMessageResponseMessage struct {
	OriginalMessage commands.SendSessionMessage
	Session         *llms.Session
	Err             *uimsg.Error
}

func (s SendCommandMessageResponseMessage) isSmokeMessage() {}

func MsgToCmd(msg tea.Msg) tea.Cmd {
	return func() tea.Msg {
		return msg
	}
}

type ModeMessage struct {
	Mode modes.Mode
}

func (m ModeMessage) isSmokeMessage() {}
