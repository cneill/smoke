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

const CommandLoad = "load"

type LoadHandler struct {
	*BaseHandler
	Path string
}

func NewLoadHandler(msg PromptCommandMessage) (Command, error) {
	load := &LoadHandler{
		BaseHandler: &BaseHandler{promptCommand: msg},
	}

	if len(msg.Args) == 0 {
		return nil, fmt.Errorf("missing path")
	}

	load.Path = msg.Args[0]

	return load, nil
}

func (l *LoadHandler) Run(_ *llms.Session) (tea.Cmd, error) {
	if l.Path == "" {
		return nil, fmt.Errorf("must provide a path to a session file")
	}

	data, err := os.ReadFile(l.Path)
	if err != nil {
		return nil, fmt.Errorf("failed to read session file %q: %w", l.Path, err)
	}

	loaded := &llms.Session{}
	if err := json.Unmarshal(data, loaded); err != nil {
		return nil, fmt.Errorf("failed to unmarshal session JSON from %q: %w", l.Path, err)
	}

	slog.Debug("loaded session from file", "path", l.Path, "num_messages", len(loaded.Messages))

	update := SessionUpdateMessage{
		PromptCommand: l.promptCommand,
		Session:       loaded,
		Message:       "Loaded session from file " + l.Path + ".",
	}

	return update.Cmd(), nil
}
