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
	includeSystem := false

	for _, arg := range msg.Args {
		if arg == "--system" {
			includeSystem = true
		}
	}

	name := session.Name
	messageCount := session.MessageCount()
	inputTokens, outputTokens := session.Usage()
	totalTokens := inputTokens + outputTokens
	duration := time.Since(session.CreatedAt)
	toolNames := session.Tools.GetTools().Names()

	info := "**Session name:** " + name + "\n\n"
	info += fmt.Sprintf("**Mode:** %s\n\n", session.GetMode())
	info += fmt.Sprintf("**Messages:** user %d, assistant %d, tool call %d\n\n",
		messageCount.UserMessages, messageCount.AssistantMessages, messageCount.ToolCallMessages)
	info += fmt.Sprintf("**Tokens:** input %d, output %d, total %d\n\n", inputTokens, outputTokens, totalTokens)
	info += fmt.Sprintf("**Duration:** %s\n\n", duration)
	info += fmt.Sprintf("**Tools available:** %s\n\n", strings.Join(toolNames, ", "))

	if includeSystem {
		info += fmt.Sprintf("\n**System message:**\n\n%s\n\n", session.SystemMessage)
	}

	// TODO: ability to get information not contained in the session object

	update := commands.HistoryUpdateMessage{
		PromptMessage: msg,
		Message:       info,
	}

	return uimsg.MsgToCmd(update), nil
}
