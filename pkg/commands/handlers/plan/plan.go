// Package plan contains a prompt command that prevents the model from making changes to files other than the plan file.
package plan

import (
	"fmt"
	"strconv"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/cneill/smoke/internal/uimsg"
	"github.com/cneill/smoke/pkg/commands"
	"github.com/cneill/smoke/pkg/llms"
	"github.com/cneill/smoke/pkg/prompts"
)

const Name = "plan"

// ModeMessage signals to Smoke to either enable or disable planning mode.
// TODO: have a single mode message that returns a session update message in a tea.Batch
type ModeMessage struct {
	commands.MessageType

	PromptMessage commands.PromptMessage
	Enabled       bool
	Message       string
	Session       *llms.Session
}

type Plan struct {
	PromptMessage commands.PromptMessage
	Enabled       bool
}

func New(msg commands.PromptMessage) (commands.Command, error) {
	handler := &Plan{
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

func (p *Plan) Name() string { return Name }

func (p *Plan) Run(session *llms.Session) (tea.Cmd, error) {
	var systemMessage, historyMessage string

	if p.Enabled {
		systemMessage = prompts.PlanningSystemPrompt().Markdown()
		historyMessage = "Entering planning mode."
	} else {
		systemMessage = prompts.WorkSystemPrompt().Markdown()
		historyMessage = "Exiting planning mode."
	}

	if err := session.SetSystemMessage(systemMessage); err != nil {
		return nil, fmt.Errorf("failed to set system message for planning mode: %w", err)
	}

	update := ModeMessage{
		PromptMessage: p.PromptMessage,
		Enabled:       p.Enabled,
		Message:       historyMessage,
		Session:       session,
	}

	return uimsg.MsgToCmd(update), nil
}
