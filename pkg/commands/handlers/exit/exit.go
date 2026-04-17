// Package exit contains a prompt command that simply exits the program.
package exit

import (
	"context"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/cneill/smoke/pkg/commands"
	"github.com/cneill/smoke/pkg/llms"
)

const Name = "exit"

type Exit struct{}

func New() (commands.Command, error) {
	return &Exit{}, nil
}

func (e *Exit) Name() string { return Name }

func (e *Exit) Help() string {
	return "Exits the program."
}

func (e *Exit) Usage() string {
	return "exit"
}

func (e *Exit) Run(_ context.Context, _ commands.PromptMessage, _ *llms.Session) (tea.Cmd, error) {
	return tea.Quit, nil
}
