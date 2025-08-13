package commands

import (
	"fmt"
	"maps"
	"slices"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/cneill/smoke/pkg/llms"
)

type Manager struct {
	ProjectPath string
	Commands    map[string]Initializer
}

func NewManager(projectPath string) *Manager {
	return &Manager{
		ProjectPath: projectPath,
		Commands: map[string]Initializer{
			CommandExit:  NewExitHandler,
			CommandSave:  NewSaveHandler,
			CommandLoad:  NewLoadHandler,
			CommandClear: NewClearHandler,
		},
	}
}

func (m *Manager) CommandNames() []string {
	results := slices.Collect(maps.Keys(m.Commands))
	slices.Sort(results)

	return results
}

func (m *Manager) Completer() func(string) []string {
	return func(input string) []string {
		results := []string{}

		if input == "" {
			return m.CommandNames()
		}

		for commandName := range m.Commands {
			if strings.HasPrefix(commandName, input) {
				results = append(results, commandName)
			}
		}

		slices.Sort(results)

		return results
	}
}

func (m *Manager) HandleCommand(session *llms.Session, msg PromptCommandMessage) (tea.Cmd, error) {
	initializer, ok := m.Commands[msg.Command]
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrUnknownCommand, msg.Command)
	}

	handler, err := initializer(msg)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrArguments, err)
	}

	cmd, err := handler.Run(session)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrRun, err)
	}

	return cmd, nil
}
