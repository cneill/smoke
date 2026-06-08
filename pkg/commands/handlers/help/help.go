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
	entries := make([]uimsg.HistoryBlock, len(h.initializers))

	idx := 0

	for name, initializer := range h.initializers {
		cmd, err := initializer()
		if err != nil {
			return nil, fmt.Errorf("failed to initialize command %q for help: %w", name, err)
		}

		entries[idx] = uimsg.HistoryBlock{
			Type:  uimsg.HistoryBlockFields,
			Title: "/" + name,
			Fields: []uimsg.HistoryField{
				uimsg.NewField("Summary", cmd.Help()),
				uimsg.NewField("Usage", "/"+cmd.Usage()),
			},
		}
		idx++
	}

	slices.SortFunc(entries, func(a, b uimsg.HistoryBlock) int {
		return strings.Compare(a.Title, b.Title)
	})

	update := commands.HistoryUpdateMessage{
		PromptMessage: msg,
		Content: &uimsg.HistoryContent{
			Blocks: entries,
		},
	}

	return uimsg.MsgToCmd(update), nil
}
