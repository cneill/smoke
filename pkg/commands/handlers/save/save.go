// Package save contains a prompt command that saves the current session to a Markdown file
package save

import (
	"bytes"
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

type Save struct {
	PromptMessage commands.PromptMessage
	Path          string
}

func New(msg commands.PromptMessage) (commands.Command, error) {
	handler := &Save{
		PromptMessage: msg,
	}

	if len(msg.Args) > 0 {
		handler.Path = msg.Args[0]
	}

	return handler, nil
}

func (s *Save) Name() string { return Name }

func (s *Save) Help() string {
	return "Saves the current session to a Markdown file."
}

func (s *Save) Usage() string {
	return "/save [path]"
}

func (s *Save) Run(session *llms.Session) (tea.Cmd, error) {
	if s.Path == "" {
		s.Path = fmt.Sprintf("%s_%s.md", session.Name, time.Now().Format(time.DateTime))
	}

	buf := &bytes.Buffer{}
	for _, msg := range session.Messages {
		buf.WriteString(msg.ToMarkdown())
	}

	slog.Debug("saving session to file as markdown", "path", s.Path, "num_messages", len(session.Messages))

	if err := os.WriteFile(s.Path, buf.Bytes(), 0o644); err != nil {
		return nil, fmt.Errorf("failed to save session to file %q: %w", s.Path, err)
	}

	update := commands.HistoryUpdateMessage{
		PromptMessage: s.PromptMessage,
		Message:       "Saved session to file " + s.Path + " in Markdown format.",
	}

	return uimsg.MsgToCmd(update), nil
}
