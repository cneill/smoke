// Package mode contains a prompt command that allows the user to change smoke's behavior/tools/system prompt.
package mode

import (
	"context"
	"fmt"
	"slices"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/cneill/smoke/internal/uimsg"
	"github.com/cneill/smoke/pkg/commands"
	"github.com/cneill/smoke/pkg/llmctx/modes"
	"github.com/cneill/smoke/pkg/llms"
	"github.com/cneill/smoke/pkg/utils"
)

const Name = "mode"

// Message signals to Smoke to change the mode it's operating in.
type Message struct {
	commands.MessageType

	PromptMessage commands.PromptMessage
	Mode          modes.Mode
	Message       string
}

type Mode struct{}

func New() (commands.Command, error) {
	return &Mode{}, nil
}

func (m *Mode) Name() string { return Name }

func (m *Mode) Help() string {
	return "Sets the 'mode' for Smoke. Different modes change the system prompt and the tools available to the model."
}

func (m *Mode) Usage() string {
	return fmt.Sprintf("mode <%s>", selectableModes("|"))
}

func (m *Mode) Run(_ context.Context, msg commands.PromptMessage, _ *llms.Session) (tea.Cmd, error) {
	if len(msg.Args) == 0 {
		return nil, fmt.Errorf("%w: must supply mode argument", commands.ErrArguments)
	}

	mode := modes.Mode(msg.Args[0])

	if !slices.Contains(modes.SelectableModes(), mode) {
		return nil, fmt.Errorf("%w: invalid mode %q, must choose one of %s", commands.ErrArguments, mode, selectableModes(", "))
	}

	result := Message{
		PromptMessage: msg,
		Mode:          mode,
		Message:       fmt.Sprintf("Entering %s mode", mode),
	}

	return uimsg.MsgToCmd(result), nil
}

func selectableModes(sep string) string {
	selectable := utils.ToStrings(modes.SelectableModes())
	slices.Sort(selectable)

	return strings.Join(selectable, sep)
}
