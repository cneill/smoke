// Package exit contains a prompt command that simply exits the program.
package exit

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/cneill/smoke/pkg/commands"
	"github.com/cneill/smoke/pkg/llms"
)

const Name = "exit"

type Exit struct {
	PromptMessage commands.PromptMessage
}

func New(msg commands.PromptMessage) (commands.Command, error) {
	handler := &Exit{
		PromptMessage: msg,
	}

	return handler, nil
}

func (e *Exit) Name() string { return Name }

func (e *Exit) Help() string {
	return "Exits the program."
}

func (e *Exit) Usage() string {
	return "/exit"
}

func (e *Exit) Run(_ *llms.Session) (tea.Cmd, error) {
	return tea.Quit, nil
}
