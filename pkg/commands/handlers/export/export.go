// Package export contains a prompt command that saves the current session to a file in JSON format that can be used with the 'load' command.
package export

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/cneill/smoke/internal/uimsg"
	"github.com/cneill/smoke/pkg/commands"
	"github.com/cneill/smoke/pkg/llms"
)

const Name = "export"

type Export struct{}

func New() (commands.Command, error) {
	return &Export{}, nil
}

func (e *Export) Name() string { return Name }

func (e *Export) Help() string {
	return "Exports the current session to a JSON file for loading later."
}

func (e *Export) Usage() string {
	return "/export [path]"
}

func (e *Export) Run(_ context.Context, msg commands.PromptMessage, session *llms.Session) (tea.Cmd, error) {
	path := fmt.Sprintf("%s_%s.json", session.Name, time.Now().Format(time.DateTime))

	if len(msg.Args) > 0 {
		path = msg.Args[0]
	}

	sessionBytes, err := json.MarshalIndent(session, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal session JSON: %w", err)
	}

	slog.Debug("exporting session to file", "path", path, "num_messages", len(session.Messages))

	if err := os.WriteFile(path, sessionBytes, 0o644); err != nil {
		return nil, fmt.Errorf("failed to export session to file %q: %w", path, err)
	}

	update := commands.HistoryUpdateMessage{
		PromptMessage: msg,
		Message:       "Exported session to file " + path + " in JSON format.",
	}

	return uimsg.MsgToCmd(update), nil
}
