// Package run contains a prompt command that runs a tool with user-specified arguments.
package run

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/cneill/smoke/internal/uimsg"
	"github.com/cneill/smoke/pkg/commands"
	"github.com/cneill/smoke/pkg/llms"
)

const Name = "run"

type Run struct{}

func New() (commands.Command, error) {
	return &Run{}, nil
}

func (r *Run) Name() string { return Name }

func (r *Run) Help() string {
	return "Runs a tool with specified arguments."
}

func (r *Run) Usage() string {
	return "/run <tool_name> <args_json>"
}

func (r *Run) Run(ctx context.Context, msg commands.PromptMessage, session *llms.Session) (tea.Cmd, error) {
	if len(msg.Args) < 2 {
		return nil, fmt.Errorf("%w: must supply tool name and arguments as JSON string", commands.ErrArguments)
	}

	toolName := msg.Args[0]
	rawArgs := strings.Join(msg.Args[1:], " ")

	args, err := session.Tools.GetArgs(toolName, []byte(rawArgs))
	if err != nil {
		return nil, fmt.Errorf("%w: %w", commands.ErrArguments, err)
	}

	// Don't want the run to stop when the Manager cancels the parent context...
	ctx = context.WithoutCancel(ctx)

	handler := func() tea.Msg {
		output, err := session.Tools.CallTool(ctx, toolName, args)
		if err != nil {
			slog.Error("tool called by run command failed", "err", err)
			return uimsg.ToError(fmt.Errorf("error running tool from prompt: %w", err))
		}

		outputMsg := llms.SimpleMessage(llms.RoleUser, output)
		if err := session.AddMessage(outputMsg); err != nil {
			return uimsg.ToError(fmt.Errorf("failed to add run message: %w", err))
		}

		update := commands.HistoryUpdateMessage{
			PromptMessage: msg,
			Message:       fmt.Sprintf("User called tool %q with args %q:\n\n%s\n", toolName, rawArgs, output),
		}

		return update
	}

	return handler, nil
}
