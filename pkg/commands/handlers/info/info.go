// Package info contains a prompt command that displays information about the current session.
package info

import (
	"context"
	"fmt"
	"slices"
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
	includeSystem := slices.Contains(msg.Args, "--system")
	messages := session.MessageCount()
	inputTokens, outputTokens := session.Usage()
	totalTokens := inputTokens + outputTokens

	toolNames := "none"
	if tools := session.Tools.GetTools(); len(tools) > 0 {
		toolNames = strings.Join(session.Tools.GetTools().Names(), ", ")
	}

	var sb strings.Builder
	sb.Grow(1024)

	fmt.Fprintf(&sb, "**Session name:** %s\n", session.Name)
	fmt.Fprintf(&sb, "**Provider:** %s\n", session.Config.Provider)
	fmt.Fprintf(&sb, "**Model:** %s\n", session.Config.Model)
	fmt.Fprintf(&sb, "**Mode:** %s\n", session.GetMode())
	fmt.Fprintf(&sb, "**Messages:** user %d, assistant %d, tool call %d\n",
		messages.UserMessages, messages.AssistantMessages, messages.ToolCallMessages)
	fmt.Fprintf(&sb, "**Tokens:** input %d, output %d, total %d\n", inputTokens, outputTokens, totalTokens)
	fmt.Fprintf(&sb, "**Duration:** %s\n", time.Since(session.CreatedAt).String())
	fmt.Fprintf(&sb, "**Tools available:** %s\n", toolNames)

	if includeSystem {
		fmt.Fprintf(&sb, "\n\n**System prompt:**\n\n%s\n", session.SystemMessage)
	}

	update := commands.HistoryUpdateMessage{
		PromptMessage: msg,
		Message:       sb.String(),
	}

	return uimsg.MsgToCmd(update), nil
}
