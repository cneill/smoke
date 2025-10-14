// Package load contains a prompt command that loads a session from a JSON file and replaces the current session with it.
package load

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/cneill/smoke/internal/uimsg"
	"github.com/cneill/smoke/pkg/commands"
	"github.com/cneill/smoke/pkg/llms"
)

const Name = "load"

type Load struct {
	PromptMessage commands.PromptMessage
	Path          string
}

func New(msg commands.PromptMessage) (commands.Command, error) {
	// Handle help generation separately
	if len(msg.Args) == 1 && msg.Args[0] == "help" {
		return &Load{PromptMessage: msg}, nil
	}

	if len(msg.Args) == 0 {
		return nil, fmt.Errorf("%w: missing path", commands.ErrArguments)
	}

	handler := &Load{
		PromptMessage: msg,
		Path:          msg.Args[0],
	}

	return handler, nil
}

func (l *Load) Name() string { return Name }

func (l *Load) Run(session *llms.Session) (tea.Cmd, error) {
	if l.Path == "" {
		return nil, fmt.Errorf("%w: must provide a path to a session file", commands.ErrArguments)
	}

	data, err := os.ReadFile(l.Path)
	if err != nil {
		return nil, fmt.Errorf("failed to read session file %q: %w", l.Path, err)
	}

	loaded := &llms.Session{}
	if err := json.Unmarshal(data, loaded); err != nil {
		return nil, fmt.Errorf("failed to unmarshal session JSON from %q: %w", l.Path, err)
	}

	loaded.Tools = session.Tools

	slog.Debug("loaded session from file", "path", l.Path, "num_messages", len(loaded.Messages))

	update := commands.SessionUpdateMessage{
		PromptMessage: l.PromptMessage,
		Session:       loaded,
		Message:       "Loaded session from file " + l.Path + ".",
		ResetHistory:  true,
	}

	return uimsg.MsgToCmd(update), nil
}

func (l *Load) Help() string {
	return "Loads a session from a JSON file and replaces the current session. Usage: /load <path>"
}
