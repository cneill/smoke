package smoke

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/cneill/smoke/pkg/llms"
)

type AssistantResponseMessage struct {
	Message *llms.Message
	Err     error
}

func (a AssistantResponseMessage) Cmd() tea.Cmd {
	return func() tea.Msg {
		return a
	}
}

type ToolCallResponseMessage struct {
	Messages []*llms.Message
	Err      error
}

func (t ToolCallResponseMessage) Cmd() tea.Cmd {
	return func() tea.Msg {
		return t
	}
}
