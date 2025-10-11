// Package session contains a prompt command that allows the user to modify the current session:
// - with the argument "new", it will start a new session without wiping the visible history
// - with the argument "clear", it will start a new session and wipe the visible history
package session

import (
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

type Session struct {
	PromptMessage commands.PromptMessage
	Command       string
}

func New(msg commands.PromptMessage) (commands.Command, error) {
	if len(msg.Args) < 1 || (msg.Args[0] != sessionNew && msg.Args[0] != sessionClear) {
		return nil, fmt.Errorf("must supply either %q or %q argument", sessionNew, sessionClear)
	}

	handler := &Session{
		PromptMessage: msg,
		Command:       msg.Args[0],
	}

	return handler, nil
}

func (s *Session) Name() string { return Name }

func (s *Session) Run(session *llms.Session) (tea.Cmd, error) {
	newSession, err := llms.NewSession(&llms.SessionOpts{
		Name:          session.Name,
		SystemMessage: session.SystemMessage,
		Tools:         session.Tools,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create new session: %w", err)
	}

	msg := "Started new LLM session"
	if s.Command == sessionClear {
		msg += " and cleared history."
	} else {
		msg += " and preserved history."
	}

	update := commands.SessionUpdateMessage{
		PromptMessage: s.PromptMessage,
		Session:       newSession,
		Message:       msg,
		ResetHistory:  s.Command == sessionClear,
	}

	return uimsg.MsgToCmd(update), nil
}
