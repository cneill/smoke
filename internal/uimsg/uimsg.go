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
