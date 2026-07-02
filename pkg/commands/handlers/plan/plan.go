// Package plan contains a prompt command for inspecting and switching the active plan log.
package plan

import (
	"context"
	"fmt"
	"slices"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/cneill/smoke/internal/uimsg"
	"github.com/cneill/smoke/pkg/commands"
	"github.com/cneill/smoke/pkg/llms"
	"github.com/cneill/smoke/pkg/utils"
)

const (
	Name = "plan"
)

type Command string

const (
	CommandCurrent = "current"
	CommandNew     = "new"
	CommandResume  = "resume"
)

func Commands() []Command {
	return []Command{CommandCurrent, CommandNew, CommandResume}
}

type Message struct {
	commands.MessageType

	Command       Command
	PromptMessage commands.PromptMessage
}

type Plan struct{}

func New() (commands.Command, error) {
	return &Plan{}, nil
}

func (p *Plan) Name() string { return Name }

func (p *Plan) Help() string {
	return "Manage the active plan log for this project."
}

func (p *Plan) Usage() string {
	return fmt.Sprintf("plan <%s>", strings.Join(utils.ToStrings(Commands()...), "|"))
}

func (p *Plan) Run(_ context.Context, msg commands.PromptMessage, _ *llms.Session) (tea.Cmd, error) {
	if len(msg.Args) != 1 {
		return nil, fmt.Errorf("%w: must supply a command", commands.ErrArguments)
	}

	cmd := Command(msg.Args[0])

	if !slices.Contains(Commands(), cmd) {
		return nil, fmt.Errorf("%w: unknown command, must supply one of %s",
			commands.ErrArguments, strings.Join(utils.ToStrings(Commands()...), ", "))
	}

	return uimsg.MsgToCmd(Message{
		Command:       cmd,
		PromptMessage: msg,
	}), nil
}
