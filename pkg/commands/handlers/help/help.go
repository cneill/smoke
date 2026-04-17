// Package help contains a prompt command that displays help for all available commands.
package help

import (
	"context"
	"fmt"
	"slices"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/cneill/smoke/internal/uimsg"
	"github.com/cneill/smoke/pkg/commands"
	"github.com/cneill/smoke/pkg/llms"
)

const Name = "help"

type Help struct {
	initializers map[string]commands.Initializer
}

func New(initializers map[string]commands.Initializer) commands.Initializer {
	return func() (commands.Command, error) {
		return &Help{
			initializers: initializers,
		}, nil
	}
}

func (h *Help) Name() string { return Name }

func (h *Help) Help() string {
	return "Shows help for all available commands."
}

func (h *Help) Usage() string {
	return "help"
}

func (h *Help) Run(_ context.Context, msg commands.PromptMessage, _ *llms.Session) (tea.Cmd, error) {
	helps := make([]string, len(h.initializers))

	idx := 0

	cmdUsage := func(name, help, usage string) string {
		return fmt.Sprintf("`/%s` - **%s**\n\t* **Usage:** `%s`", name, help, usage)
	}

	for name, initializer := range h.initializers {
		cmd, err := initializer()
		if err != nil {
			return nil, fmt.Errorf("failed to initialize command %q for help: %w", name, err)
		}

		helps[idx] = cmdUsage(name, cmd.Help(), cmd.Usage())
		idx++
	}

	slices.Sort(helps)

	builder := &strings.Builder{}

	for _, help := range helps {
		fmt.Fprintf(builder, "* %s\n", help)
	}

	update := commands.HistoryUpdateMessage{
		PromptMessage: msg,
		Message:       builder.String(),
	}

	return uimsg.MsgToCmd(update), nil
}
