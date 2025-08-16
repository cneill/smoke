package commands

import (
	"bytes"
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

// CommandClear clears the current session contents.
const CommandClear = "clear"

type ClearHandler struct {
	*BaseHandler
}

func NewClearHandler(msg PromptCommandMessage) (Command, error) {
	handler := &ClearHandler{
		BaseHandler: &BaseHandler{promptCommand: msg},
	}

	return handler, nil
}

func (c *ClearHandler) Run(session *llms.Session) (tea.Cmd, error) {
	newSession, err := llms.NewSession(&llms.SessionOpts{
		Name:          session.Name,
		SystemMessage: session.SystemMessage,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to clear session and create new one: %w", err)
	}

	update := SessionUpdateMessage{
		PromptCommand: c.promptCommand,
		Session:       newSession,
		Message:       "Cleared session.",
	}

	return update.Cmd(), nil
}

// CommandExit closes the program.
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

// CommandExport saves the current session to a file in JSON format that can be used with the 'load' command.
const CommandExport = "export"

type ExportHandler struct {
	*BaseHandler

	Path string
}

func NewExportHandler(msg PromptCommandMessage) (Command, error) {
	export := &ExportHandler{
		BaseHandler: &BaseHandler{
			promptCommand: msg,
		},
	}
	if len(msg.Args) > 0 {
		export.Path = msg.Args[0]
	}

	return export, nil
}

func (e *ExportHandler) Run(session *llms.Session) (tea.Cmd, error) {
	if e.Path == "" {
		e.Path = fmt.Sprintf("%s_%s.json", session.Name, time.Now().Format(time.DateTime))
	}

	sessionBytes, err := json.MarshalIndent(session, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal session JSON: %w", err)
	}

	slog.Debug("exporting session to file", "path", e.Path, "num_messages", len(session.Messages))

	if err := os.WriteFile(e.Path, sessionBytes, 0o644); err != nil {
		return nil, fmt.Errorf("failed to export session to file %q: %w", e.Path, err)
	}

	update := HistoryUpdateMessage{
		PromptCommand: e.promptCommand,
		Message:       "Exported session to file " + e.Path + " in JSON format.",
	}

	return update.Cmd(), nil
}

// CommandLoad loads a session from a JSON file and replaces the current session with it.
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

// CommandSave saves the current session to a Markdown file
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

	update := HistoryUpdateMessage{
		PromptCommand: s.promptCommand,
		Message:       "Saved session to file " + s.Path + " in Markdown format.",
	}

	return update.Cmd(), nil
}
