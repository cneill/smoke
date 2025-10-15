// Package help contains a prompt command that displays help for all available commands.
package help

import (
	"fmt"
	"log/slog"
	"slices"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/cneill/smoke/internal/uimsg"
	"github.com/cneill/smoke/pkg/commands"
	"github.com/cneill/smoke/pkg/llms"
)

const Name = "help"

type Help struct {
	PromptMessage commands.PromptMessage
	initializers  map[string]commands.Initializer
}

func New(initializers map[string]commands.Initializer) commands.Initializer {
	return func(msg commands.PromptMessage) (commands.Command, error) {
		return &Help{
			PromptMessage: msg,
			initializers:  initializers,
		}, nil
	}
}

func (h *Help) Name() string { return Name }

func (h *Help) Help() string { return "Shows help for all available commands." }

func (h *Help) Run(_ *llms.Session) (tea.Cmd, error) {
	helps := make([]string, len(h.initializers)+1)

	idx := 0

	for name, init := range h.initializers {
		cmd, err := init(commands.PromptMessage{Command: name, Args: []string{"help"}})
		if err != nil {
			slog.Error("failed to initialize command for help generation", "command", name, "error", err)
			continue
		}

		helps[idx] = fmt.Sprintf("/%s - %s", name, cmd.Help())
		idx++
	}

	helps[idx] = fmt.Sprintf("/%s - %s", h.Name(), h.Help())

	slices.Sort(helps)

	builder := &strings.Builder{}

	for _, help := range helps {
		fmt.Fprintf(builder, "* %s\n", help)
	}

	return uimsg.MsgToCmd(commands.HistoryUpdateMessage{
		PromptMessage: h.PromptMessage,
		Message:       builder.String(),
	}), nil
}
