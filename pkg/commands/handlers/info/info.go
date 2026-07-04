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
	usage := session.GetUsage()
	totalTokens := usage.TotalInputTokens + usage.TotalOutputTokens

	toolNames := "none"
	if tools := session.Tools.GetTools(); len(tools) > 0 {
		toolNames = strings.Join(session.Tools.GetTools().Names(), ", ")
	}

	content := &uimsg.HistoryContent{
		Blocks: []uimsg.HistoryBlock{
			{
				Type:  uimsg.HistoryBlockFields,
				Title: "Session info",
				Fields: []uimsg.HistoryField{
					uimsg.NewField("Session name", session.Name),
					uimsg.NewField("Provider", string(session.Config.Provider)),
					uimsg.NewField("Model", session.Config.Model),
					uimsg.NewField("Reasoning Effort", session.Config.Effort),
					uimsg.NewField("Mode", string(session.GetMode())),
					uimsg.NewField(
						"Messages",
						fmt.Sprintf("User=%d, Assistant=%d, Tool call=%d",
							messages.UserMessages, messages.AssistantMessages, messages.ToolCallMessages),
					),
					uimsg.NewField(
						"Total session API usage",
						fmt.Sprintf("Input=%d, Output=%d, Total=%d", usage.TotalInputTokens, usage.TotalOutputTokens, totalTokens),
					),
					uimsg.NewField(
						"Current context window usage",
						fmt.Sprintf("Tokens=%d", usage.CurrentContextWindowTokens),
					),
					uimsg.NewField("Duration", time.Since(session.CreatedAt).String()),
					uimsg.NewField("Tools available", toolNames),
				},
			},
		},
	}

	if includeSystem {
		content.Blocks = append(content.Blocks, uimsg.HistoryBlock{
			Type:  uimsg.HistoryBlockMarkdown,
			Title: "System prompt",
			Text:  session.SystemMessage,
		})
	}

	update := commands.HistoryUpdateMessage{
		PromptMessage: msg,
		Content:       content,
	}

	return uimsg.MsgToCmd(update), nil
}
