// Package help contains a prompt command that displays help for all available commands.
package help

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/cneill/smoke/internal/uimsg"
	"github.com/cneill/smoke/pkg/commands"
	"github.com/cneill/smoke/pkg/llms"
)

const Name = "help"

type Help struct {
	PromptMessage commands.PromptMessage
	manager       *commands.Manager
}

func New(manager *commands.Manager) commands.Initializer {
	return func(msg commands.PromptMessage) (commands.Command, error) {
		return &Help{
			PromptMessage: msg,
			manager:       manager,
		}, nil
	}
}

func (h *Help) Name() string { return Name }

func (h *Help) Help() string { return "Shows help for all available commands." }

func (h *Help) Run(_ *llms.Session) (tea.Cmd, error) {
	return uimsg.MsgToCmd(commands.HistoryUpdateMessage{
		PromptMessage: h.PromptMessage,
		Message:       h.manager.Help(),
	}), nil
}
