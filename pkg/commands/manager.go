package commands

import (
	"fmt"
	"log/slog"
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
	manager := &Manager{
		ProjectPath: projectPath,
		Commands:    map[string]Initializer{},
	}

	return manager
}

func (m *Manager) Register(name string, initializer Initializer) {
	m.Commands[name] = initializer
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

func (m *Manager) HandleCommand(session *llms.Session, msg PromptMessage) (tea.Cmd, error) {
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

	slog.Debug("ran prompt command", "command", msg.Command, "args", msg.Args)

	return cmd, nil
}

func (m *Manager) Help() string {
	helps := make([]string, len(m.Commands))

	idx := 0

	for name, init := range m.Commands {
		cmd, err := init(PromptMessage{Command: name, Args: []string{"help"}})
		if err != nil {
			slog.Error("failed to initialize command for help generation", "command", name, "error", err)
			continue
		}

		helps[idx] = fmt.Sprintf("/%s %s", name, cmd.Help())
		idx++
	}

	slices.Sort(helps)

	builder := &strings.Builder{}

	for _, help := range helps {
		fmt.Fprintf(builder, "* %s\n", help)
	}

	return builder.String()
}
