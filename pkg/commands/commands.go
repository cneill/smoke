// Package commands handles "/" commands entered by the user in the prompt box. These can include things like exiting
// the program or saving the current session to a file.
package commands

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/cneill/smoke/pkg/llms"
)

type Command interface {
	Name() string
	Run(session *llms.Session) (tea.Cmd, error)
}

type Initializer func(msg PromptMessage) (Command, error)
