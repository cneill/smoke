// Package save contains a prompt command that saves the current session to a Markdown file
package save

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"os"
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
	return "Saves the current session to a Markdown file."
}

func (s *Save) Usage() string {
	return "save [path]"
}

func (s *Save) Run(_ context.Context, msg commands.PromptMessage, session *llms.Session) (tea.Cmd, error) {
	path := fmt.Sprintf("%s_%s.md", session.Name, time.Now().Format(time.DateTime))

	if len(msg.Args) > 0 {
		path = msg.Args[0]
	}

	buf := &bytes.Buffer{}
	for _, msg := range session.Messages {
		buf.WriteString(msg.ToMarkdown())
	}

	slog.Debug("saving session to file as markdown", "path", path, "num_messages", len(session.Messages))

	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		return nil, fmt.Errorf("failed to save session to file %q: %w", path, err)
	}

	update := commands.HistoryUpdateMessage{
		PromptMessage: msg,
		Message:       "Saved session to file " + path + " in Markdown format.",
	}

	return uimsg.MsgToCmd(update), nil
}
