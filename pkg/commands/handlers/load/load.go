// Package load contains a prompt command that loads a session from a JSON file and replaces the current session with it.
package load

import (
	"context"
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

type Load struct{}

func New() (commands.Command, error) {
	return &Load{}, nil
}

func (l *Load) Name() string { return Name }

func (l *Load) Help() string {
	return "Loads a session from a JSON file and replaces the current session."
}

func (l *Load) Usage() string {
	return "load <path>"
}

func (l *Load) Run(_ context.Context, msg commands.PromptMessage, session *llms.Session) (tea.Cmd, error) {
	if len(msg.Args) == 0 || msg.Args[0] == "" {
		return nil, fmt.Errorf("%w: missing path to a session file", commands.ErrArguments)
	}

	path := msg.Args[0]

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read session file %q: %w", path, err)
	}

	loaded := &llms.Session{}
	if err := json.Unmarshal(data, loaded); err != nil {
		return nil, fmt.Errorf("failed to unmarshal session JSON from %q: %w", path, err)
	}

	// TODO: there's almost certainly more setup necessary here...
	loaded.Tools = session.Tools

	slog.Debug("loaded session from file", "path", path, "num_messages", len(loaded.Messages))

	update := commands.SessionUpdateMessage{
		PromptMessage: msg,
		Session:       loaded,
		Message:       "Loaded session from file " + path + ".",
		ResetHistory:  true,
	}

	return uimsg.MsgToCmd(update), nil
}
