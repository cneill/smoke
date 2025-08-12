package commands

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/cneill/smoke/pkg/llms"
)

type BaseHandler struct {
	promptCommand PromptCommandMessage
}

const CommandExit = "exit"

type ExitHandler struct {
	*BaseHandler
}

func NewExitHandler(msg PromptCommandMessage) (Command, error) {
	handler := &ExitHandler{
		&BaseHandler{
			promptCommand: msg,
		},
	}

	return handler, nil
}

func (e *ExitHandler) Run(_ *llms.Session) (tea.Cmd, error) {
	return tea.Quit, nil
}

const CommandSave = "save"

type SaveHandler struct {
	*BaseHandler
	Path string
}

func NewSaveHandler(msg PromptCommandMessage) (Command, error) {
	save := &SaveHandler{
		BaseHandler: &BaseHandler{
			promptCommand: msg,
		},
	}
	if len(msg.Args) > 0 {
		save.Path = msg.Args[0]
	}

	return save, nil
}

func (s *SaveHandler) Run(session *llms.Session) (tea.Cmd, error) {
	if s.Path == "" {
		s.Path = fmt.Sprintf("%s_saved_%s.json", session.Name, time.Now().Format(time.DateTime))
	}

	sessionBytes, err := json.MarshalIndent(session, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal session JSON: %w", err)
	}

	slog.Debug("saving session to file", "path", s.Path, "num_messages", len(session.Messages))

	if err := os.WriteFile(s.Path, sessionBytes, 0o644); err != nil {
		return nil, fmt.Errorf("failed to write session to file %q: %w", s.Path, err)
	}

	update := HistoryUpdateMessage{
		PromptCommand: s.promptCommand,
		Message:       "Saved session to file " + s.Path + ".",
	}

	return update.Cmd(), nil
}
