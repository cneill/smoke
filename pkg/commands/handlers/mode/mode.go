// Package review contains a prompt command that asks the model to review the code referenced by the user for red flags
package mode

import (
	"fmt"
	"slices"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/cneill/smoke/internal/uimsg"
	"github.com/cneill/smoke/pkg/commands"
	"github.com/cneill/smoke/pkg/llms"
	"github.com/cneill/smoke/pkg/tools"
)

const Name = "mode"

// Message signals to Smoke to change the mode it's operating in.
type Message struct {
	commands.MessageType

	PromptMessage commands.PromptMessage
	Mode          llms.Mode
	Message       string
	// Session       *llms.Session
}

type Mode struct {
	PromptMessage commands.PromptMessage
	mode          llms.Mode
}

func New(msg commands.PromptMessage) (commands.Command, error) {
	// Handle help generation separately
	if len(msg.Args) == 1 && msg.Args[0] == "help" {
		return &Mode{PromptMessage: msg}, nil
	}

	if len(msg.Args) == 0 {
		return nil, fmt.Errorf("%w: must supply mode argument", commands.ErrArguments)
	}

	handler := &Mode{
		PromptMessage: msg,
	}

	mode := llms.Mode(msg.Args[0])

	if !slices.Contains(llms.SelectableModes(), mode) {
		return nil, fmt.Errorf("invalid mode %q, must choose one of %s", mode, selectableModes(", "))
	}

	handler.mode = mode

	return handler, nil
}

func (m *Mode) Name() string { return Name }

func (m *Mode) Help() string {
	return "Sets the 'mode' for Smoke. Different modes change the system prompt and the tools available to the model."
}

func (m *Mode) Usage() string {
	return fmt.Sprintf("/mode <%s>", selectableModes("|"))
}

func (m *Mode) Run(_ *llms.Session) (tea.Cmd, error) {
	historyMessage := fmt.Sprintf("Entering %s mode", m.mode)
	msg := Message{
		PromptMessage: m.PromptMessage,
		Mode:          m.mode,
		Message:       historyMessage,
	}

	return uimsg.MsgToCmd(msg), nil
}

func selectableModes(sep string) string {
	// TODO: move tools.ToStrings somewhere else - doesn't make sense to pull that package in here
	selectable := tools.ToStrings(llms.SelectableModes())
	slices.Sort(selectable)

	return strings.Join(selectable, sep)
}
