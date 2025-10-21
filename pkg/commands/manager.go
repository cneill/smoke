package commands

import (
	"context"
	"fmt"
	"log/slog"
	"maps"
	"slices"
	"strings"
	"sync"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/cneill/smoke/internal/uimsg"
	"github.com/cneill/smoke/pkg/llms"
)

type Manager struct {
	ProjectPath  string
	initializers map[string]Initializer
	commands     map[string]Command
	mutex        sync.RWMutex

	teaEmitter uimsg.TeaEmitter
}

func NewManager(projectPath string) *Manager {
	manager := &Manager{
		ProjectPath:  projectPath,
		initializers: map[string]Initializer{},
		commands:     map[string]Command{},
		mutex:        sync.RWMutex{},
	}

	return manager
}

func (m *Manager) Register(name string, initializer Initializer) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	m.initializers[name] = initializer

	cmd, err := initializer()
	if err != nil {
		return fmt.Errorf("failed to initialize command %q: %w", name, err)
	}

	if m.teaEmitter != nil {
		if wte, ok := cmd.(WantsTeaEmitter); ok {
			wte.SetTeaEmitter(m.teaEmitter)
		}
	}

	m.commands[name] = cmd

	return nil
}

func (m *Manager) SetTeaEmitter(emitter uimsg.TeaEmitter) {
	m.teaEmitter = emitter

	m.mutex.RLock()
	defer m.mutex.RUnlock()

	for _, cmd := range m.commands {
		if wte, ok := cmd.(WantsTeaEmitter); ok {
			wte.SetTeaEmitter(emitter)
		}
	}
}

func (m *Manager) CommandNames() []string {
	results := slices.Collect(maps.Keys(m.initializers))
	slices.Sort(results)

	return results
}

func (m *Manager) Completer() func(string) []string {
	return func(input string) []string {
		results := []string{}

		m.mutex.RLock()
		defer m.mutex.RUnlock()

		for name, cmd := range m.commands {
			if strings.HasPrefix(name, input) || input == "" {
				results = append(results, cmd.Usage())
			}
		}

		slices.Sort(results)

		return results
	}
}

func (m *Manager) HandleCommand(session *llms.Session, msg PromptMessage) (tea.Cmd, error) {
	slog.Debug("running prompt command", "command", msg.Command, "args", msg.Args)

	m.mutex.RLock()
	defer m.mutex.RUnlock()

	handler, ok := m.commands[msg.Command]
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrUnknownCommand, msg.Command)
	}

	// TODO: timeout?
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cmd, err := handler.Run(ctx, msg, session)
	if err != nil {
		return nil, fmt.Errorf("error running prompt command %q: %w", msg.Command, err)
	}

	slog.Debug("prompt command executed successfully", "command", msg.Command, "args", msg.Args)

	return cmd, nil
}
