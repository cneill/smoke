package uimsg

import (
	tea "github.com/charmbracelet/bubbletea"
)

// MsgToCmd is a simple helper function to turn any message into a tea.Cmd that returns that message as a tea.Msg.
func MsgToCmd(msg any) tea.Cmd {
	return func() tea.Msg {
		return msg
	}
}

// TeaEmitter is a function that pushes messages directly to the main Bubbletea event loop.
type TeaEmitter func(tea.Msg)

// Error is for generalized errors reported to the UI.
type Error struct {
	Err error
}

func ToError(err error) *Error {
	return &Error{Err: err}
}

func (e *Error) Error() string {
	if e == nil || e.Err == nil {
		return ""
	}

	return e.Err.Error()
}

func ErrCmd(err error) tea.Cmd {
	return MsgToCmd(ToError(err))
}
