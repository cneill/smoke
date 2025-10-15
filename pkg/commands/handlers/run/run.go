// Package run contains a prompt command that runs a tool with user-specified arguments.
package run

import (
	"context"
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/cneill/smoke/internal/uimsg"
	"github.com/cneill/smoke/pkg/commands"
	"github.com/cneill/smoke/pkg/llms"
)

const Name = "run"

type Run struct {
	PromptMessage commands.PromptMessage
	ToolName      string
	RawArgs       string
}

func New(msg commands.PromptMessage) (commands.Command, error) {
	// Handle help generation separately
	if len(msg.Args) == 1 && msg.Args[0] == "help" {
		return &Run{PromptMessage: msg}, nil
	}

	if len(msg.Args) < 2 {
		return nil, fmt.Errorf("must supply tool name and arguments as JSON string")
	}

	handler := &Run{
		PromptMessage: msg,
		ToolName:      msg.Args[0],
		RawArgs:       strings.Join(msg.Args[1:], " "),
	}

	return handler, nil
}

func (r *Run) Name() string { return Name }

func (r *Run) Help() string {
	return "Runs a tool with specified arguments."
}

func (r *Run) Usage() string {
	return "/run <tool_name> <args_json>"
}

func (r *Run) Run(session *llms.Session) (tea.Cmd, error) {
	args, err := session.Tools.GetArgs(r.ToolName, []byte(r.RawArgs))
	if err != nil {
		return nil, fmt.Errorf("%w: %w", commands.ErrArguments, err)
	}

	output, err := session.Tools.CallTool(context.TODO(), r.ToolName, args)
	if err != nil {
		return nil, fmt.Errorf("error running tool from prompt: %w", err)
	}

	msg := llms.SimpleMessage(llms.RoleUser, output)
	if err := session.AddMessage(msg); err != nil {
		return nil, fmt.Errorf("failed to add run message: %w", err)
	}

	updateMsg := fmt.Sprintf("User called tool %q with args %q:\n\n%s\n", r.ToolName, r.RawArgs, output)

	update := commands.HistoryUpdateMessage{
		PromptMessage: r.PromptMessage,
		Message:       updateMsg,
	}

	return uimsg.MsgToCmd(update), nil
}
