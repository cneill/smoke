// Package review contains a prompt command that asks the model to review the code referenced by the user for red flags
package review

import (
	"fmt"
	"strconv"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/cneill/smoke/internal/uimsg"
	"github.com/cneill/smoke/pkg/commands"
	"github.com/cneill/smoke/pkg/llms"
	"github.com/cneill/smoke/pkg/prompts"
)

const Name = "review"

// ModeMessage signals to Smoke to either enable or disable review mode.
// TODO: have a single mode message that returns a session update message in a tea.Batch
type ModeMessage struct {
	commands.MessageType

	PromptMessage commands.PromptMessage
	Enabled       bool
	Message       string
	Session       *llms.Session
}

type Review struct {
	PromptMessage commands.PromptMessage
	Enabled       bool
}

func New(msg commands.PromptMessage) (commands.Command, error) {
	// Handle help generation separately
	if len(msg.Args) == 1 && msg.Args[0] == "help" {
		return &Review{PromptMessage: msg}, nil
	}

	handler := &Review{
		PromptMessage: msg,
	}

	if len(msg.Args) == 0 {
		handler.Enabled = true
		return handler, nil
	}

	boolVal, err := strconv.ParseBool(msg.Args[0])
	if err != nil {
		switch msg.Args[0] {
		case "off":
			boolVal = false
		case "on":
			boolVal = true
		default:
			return nil, fmt.Errorf("%w: %s", commands.ErrArguments, msg.Args[0])
		}
	}

	handler.Enabled = boolVal

	return handler, nil
}

func (r *Review) Name() string { return Name }

func (r *Review) Run(session *llms.Session) (tea.Cmd, error) {
	var systemMessage, historyMessage string

	if r.Enabled {
		systemMessage = prompts.ReviewSystemPrompt().Markdown()
		historyMessage = "Entering review mode."
	} else {
		systemMessage = prompts.WorkSystemPrompt().Markdown()
		historyMessage = "Exiting review mode."
	}

	if err := session.SetSystemMessage(systemMessage); err != nil {
		return nil, fmt.Errorf("failed to set system message for review mode: %w", err)
	}

	update := ModeMessage{
		PromptMessage: r.PromptMessage,
		Enabled:       r.Enabled,
		Message:       historyMessage,
		Session:       session,
	}

	return uimsg.MsgToCmd(update), nil
}

func (r *Review) Help() string {
	return "Enables or disables review mode, where the model checks for code issues. Usage: /review [on|off]"
}
