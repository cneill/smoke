// Package info contains a prompt command that displays information about the current session.
package info

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/cneill/smoke/internal/uimsg"
	"github.com/cneill/smoke/pkg/commands"
	"github.com/cneill/smoke/pkg/llms"
)

const Name = "info"

type Info struct{}

func New() (commands.Command, error) {
	return &Info{}, nil
}

func (i *Info) Name() string { return Name }

func (i *Info) Help() string {
	return "Displays detailed information about the current session, including message and token counts, tools, and " +
		"more. Use --system to include the current system prompt."
}

func (i *Info) Usage() string {
	return "info [--system]"
}

func (i *Info) Run(_ context.Context, msg commands.PromptMessage, session *llms.Session) (tea.Cmd, error) {
	builder := &strings.Builder{}
	builder.Grow(1024)

	includeSystem := false

	for _, arg := range msg.Args {
		if arg == "--system" {
			includeSystem = true
		}
	}

	messageCount := session.MessageCount()
	inputTokens, outputTokens := session.Usage()
	totalTokens := inputTokens + outputTokens
	toolNames := strings.Join(session.Tools.GetTools().Names(), ", ")

	builder.WriteString("**Session name:** " + session.Name + "\n")
	builder.WriteString("**Mode:** " + string(session.GetMode()) + "\n")
	fmt.Fprintf(builder, "**Messages:** user %d, assistant %d, tool call %d\n",
		messageCount.UserMessages, messageCount.AssistantMessages, messageCount.ToolCallMessages)
	fmt.Fprintf(builder, "**Tokens:** input %d, output %d, total %d\n", inputTokens, outputTokens, totalTokens)
	builder.WriteString("**Duration:** " + time.Since(session.CreatedAt).String() + "\n")
	builder.WriteString("**Tools available:** " + toolNames + "\n")

	if includeSystem {
		builder.WriteString("\n\n****System prompt:**\n" + session.SystemMessage + "\n")
	}

	update := commands.HistoryUpdateMessage{
		PromptMessage: msg,
		Message:       builder.String(),
	}

	return uimsg.MsgToCmd(update), nil
}
