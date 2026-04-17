// Package session contains a prompt command that allows the user to modify the current session:
// - with the argument "new", it will start a new session without wiping the visible history
// - with the argument "clear", it will start a new session and wipe the visible history
package session

import (
	"context"
	"fmt"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/cneill/smoke/internal/uimsg"
	"github.com/cneill/smoke/pkg/commands"
	"github.com/cneill/smoke/pkg/llms"
)

const (
	Name = "session"

	sessionNew   = "new"
	sessionClear = "clear"
)

type Session struct{}

func New() (commands.Command, error) {
	return &Session{}, nil
}

func (s *Session) Name() string { return Name }

func (s *Session) Help() string {
	return "Create a new session and optionally replace the existing history in the terminal (with 'clear')."
}

func (s *Session) Usage() string {
	return "session <new|clear>"
}

func (s *Session) Run(_ context.Context, msg commands.PromptMessage, session *llms.Session) (tea.Cmd, error) {
	if len(msg.Args) < 1 || (msg.Args[0] != sessionNew && msg.Args[0] != sessionClear) {
		return nil, fmt.Errorf("%w: must supply either %q or %q argument", commands.ErrArguments, sessionNew, sessionClear)
	}

	command := msg.Args[0]

	newSession, err := llms.NewSession(&llms.SessionOpts{
		Name:            session.Name,
		SystemMessage:   session.SystemMessage,
		Tools:           session.Tools,
		SystemAsMessage: session.SystemAsMessage,
		Mode:            session.GetMode(),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create new session: %w", err)
	}

	historyMsg := "Started new LLM session"
	if command == sessionClear {
		historyMsg += " and cleared history."
	} else {
		historyMsg += " and preserved history."
	}

	update := commands.SessionUpdateMessage{
		PromptMessage: msg,
		Session:       newSession,
		Message:       historyMsg,
		ResetHistory:  command == sessionClear,
	}

	return uimsg.MsgToCmd(update), nil
}
