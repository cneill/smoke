// Package save contains a prompt command that saves the current session to a file in JSON format that can be used with the 'load' command.
package save

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/cneill/smoke/internal/uimsg"
	"github.com/cneill/smoke/pkg/commands"
	"github.com/cneill/smoke/pkg/llms"
)

const Name = "save"

type Save struct{}

func New() (commands.Command, error) {
	return &Save{}, nil
}

func (s *Save) Name() string { return Name }

func (s *Save) Help() string {
	return "Saves the current session to a JSON file for loading with /load later."
}

func (s *Save) Usage() string {
	return "save [path]"
}

func (s *Save) Run(_ context.Context, msg commands.PromptMessage, session *llms.Session) (tea.Cmd, error) {
	path := fmt.Sprintf("%s_%s.json", session.Name, time.Now().Format(time.DateTime))

	if len(msg.Args) > 0 {
		path = msg.Args[0]
	}

	sessionBytes, err := json.MarshalIndent(session, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal session JSON: %w", err)
	}

	slog.Debug("saving session to file", "path", path, "num_messages", len(session.Messages))

	if err := os.WriteFile(path, sessionBytes, 0o644); err != nil {
		return nil, fmt.Errorf("failed to save session to file %q: %w", path, err)
	}

	content := &uimsg.HistoryContent{
		Blocks: []uimsg.HistoryBlock{
			{
				Type:  uimsg.HistoryBlockFields,
				Title: "Save complete",
				Fields: []uimsg.HistoryField{
					uimsg.NewField("Path", path),
					uimsg.NewField("Format", "JSON"),
					uimsg.NewField("Messages", strconv.Itoa(len(session.Messages))),
				},
			},
		},
	}

	update := commands.HistoryUpdateMessage{
		PromptMessage: msg,
		Content:       content,
	}

	return uimsg.MsgToCmd(update), nil
}
