package commands

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/cneill/smoke/pkg/llms"
)

type Command interface {
	Run(session *llms.Session) (tea.Cmd, error)
}

type Initializer func(msg PromptCommandMessage) (Command, error)
