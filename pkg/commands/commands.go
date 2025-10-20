// Package commands handles "/" commands entered by the user in the prompt box. These can include things like exiting
// the program or saving the current session to a file.
package commands

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/cneill/smoke/internal/uimsg"
	"github.com/cneill/smoke/pkg/llms"
)

type Command interface {
	Name() string
	Help() string
	Usage() string
	Run(session *llms.Session) (tea.Cmd, error)
}

type Initializer func(msg PromptMessage) (Command, error)

type WantsTeaEmitter interface {
	Command

	SetTeaEmitter(emitter uimsg.TeaEmitter)
}
