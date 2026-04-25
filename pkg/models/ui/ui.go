// Package ui contains a Bubbletea model that wraps other models like [history.Model] and [input.Model], as well as the
// [*smoke.Smoke] struct that contains and modifies application state. It is the main model for the application,
// executed as part of the main() function.
package ui

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/cneill/smoke/internal/uimsg"
	"github.com/cneill/smoke/pkg/commands"
	"github.com/cneill/smoke/pkg/commands/handlers/edit"
	"github.com/cneill/smoke/pkg/commands/handlers/mode"
	"github.com/cneill/smoke/pkg/commands/handlers/rank"
	"github.com/cneill/smoke/pkg/commands/handlers/summarize"
	"github.com/cneill/smoke/pkg/elicit"
	"github.com/cneill/smoke/pkg/llms"
	"github.com/cneill/smoke/pkg/models/banner"
	"github.com/cneill/smoke/pkg/models/history"
	"github.com/cneill/smoke/pkg/models/input"
	"github.com/cneill/smoke/pkg/smoke"
	"golang.org/x/term"
)

const gap = "\n"

type Opts struct {
	Smoke *smoke.Smoke
}

func (o *Opts) OK() error {
	if o.Smoke == nil {
		return fmt.Errorf("missing smoke")
	}

	return nil
}

type Model struct {
	smoke *smoke.Smoke

	banner  *banner.Model
	history *history.Model
	input   *input.Model
}

func New(opts *Opts) (*Model, error) {
	if err := opts.OK(); err != nil {
		return nil, fmt.Errorf("options error: %w", err)
	}

	width, height, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil {
		return nil, fmt.Errorf("failed to get terminal size: %w", err)
	}

	bannerModel := banner.New()

	historyOpts := &history.Opts{
		Width:       width,
		Height:      height - 2,
		InitContent: bannerModel.View(),
	}

	historyModel, err := history.New(historyOpts)
	if err != nil {
		return nil, fmt.Errorf("failed to set up history view: %w", err)
	}

	inputOpts := &input.Opts{
		Width:            width,
		Height:           2,
		MaxHeight:        5,
		PlaceholderText:  "Enter your message...",
		CommandCompleter: opts.Smoke.CommandCompleter(),
		SkillCompleter:   opts.Smoke.SkillCompleter(),
	}

	inputModel, err := input.New(inputOpts)
	if err != nil {
		return nil, fmt.Errorf("failed to set up input view: %w", err)
	}

	model := &Model{
		smoke: opts.Smoke,

		banner:  bannerModel,
		history: historyModel,
		input:   inputModel,
	}

	return model, nil
}

func (m *Model) Init() tea.Cmd {
	cmds := tea.Batch(m.history.Init(), m.input.Init())
	return cmds
}

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	cmds := []tea.Cmd{}

	inputModel, inputCmd := m.input.Update(msg)
	m.input = inputModel

	if inputCmd != nil {
		cmds = append(cmds, inputCmd)
	}

	// don't send key messages to history unless the input is unfocused or waiting
	if _, ok := msg.(tea.KeyMsg); !ok || ok && (!m.input.Focused() || m.input.Waiting()) {
		historyModel, historyCmd := m.history.Update(msg)
		m.history = historyModel

		if historyCmd != nil {
			cmds = append(cmds, historyCmd)
		}
	}

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		lineHeight := m.input.LineHeight()
		m.history.Resize(msg.Width, msg.Height-(lineHeight+1)) // +1 for the border
		m.input.Resize(msg.Width, lineHeight)

		// m.resize(msg)
	case tea.KeyMsg:
		switch msg.Type { //nolint:exhaustive,gocritic
		case tea.KeyCtrlC:
			return m, tea.Quit
		}
	case input.Message:
		cmds = append(cmds, m.handleInputMessage(msg))
	case commands.Message:
		cmds = append(cmds, m.handleCommandMessage(msg))
	case smoke.Message:
		cmds = append(cmds, m.handleSmokeMessage(msg))
	case elicit.Message:
		cmds = append(cmds, m.handleElicitMessage(msg))
	case *uimsg.Error:
		slog.Error("got raw error in ui event loop", "err", msg.Err)
		cmds = append(cmds, updateHistory(msg))
	}

	return m, tea.Batch(cmds...)
}

func (m *Model) View() string {
	return fmt.Sprintf("%s%s%s", m.history.View(), gap, m.input.View())
}

// Handle messages coming from the input bubbletea model.
func (m *Model) handleInputMessage(msg input.Message) tea.Cmd {
	cmds := []tea.Cmd{}

	switch msg := msg.(type) {
	case input.ResizeMessage:
		lineHeight := m.input.LineHeight()
		delta := lineHeight - m.input.GetHeight() // how many lines did we resize by
		width := m.history.GetWidth()
		m.history.Resize(width, m.history.GetHeight()-delta)
		m.input.Resize(width, lineHeight)

	case input.ShiftModeMessage:
		if err := m.smoke.ShiftMode(); err != nil {
			return updateHistory(err)
		}

	case input.UserMessage:
		llmMessage := llms.SimpleMessage(llms.RoleUser, msg.Content)
		cmds = append(cmds, updateHistory(llmMessage))
		cmds = append(cmds, m.input.SetWaiting(true))

		cmd, err := m.smoke.HandleUserMessage(llmMessage)
		if err != nil {
			return updateHistory(err)
		}

		cmds = append(cmds, cmd)

	case input.CancelUserMessage:
		m.smoke.CancelUserMessage(msg.Err)
	}

	return tea.Batch(cmds...)
}

// Handle messages coming from the main Smoke controller.
func (m *Model) handleSmokeMessage(msg smoke.Message) tea.Cmd {
	cmds := []tea.Cmd{}

	switch msg := msg.(type) {
	case smoke.AssistantResponseMessage:
		cmds = append(cmds, m.handleAssistantResponse(msg))
	case smoke.AssistantUpdatedStreamMessage:
		cmds = append(cmds, m.handleAssistantUpdatedStream(msg))
	case smoke.ToolCallResponseMessage:
		cmds = append(cmds, m.handleToolCallResponse(msg))
	case smoke.UsageUpdateMessage, smoke.ModeMessage:
		newInput, cmd := m.input.Update(msg)
		m.input = newInput

		cmds = append(cmds, cmd)
	}

	return tea.Batch(cmds...)
}

func (m *Model) handleElicitMessage(msg elicit.Message) tea.Cmd {
	cmds := []tea.Cmd{}

	switch msg := msg.(type) {
	case elicit.RequestMessage:
		cmds = append(cmds, m.input.SetWaiting(false))
		cmds = append(cmds, updateHistory(msg))

		if err := m.input.BeginElicit(msg); err != nil {
			cmds = append(cmds, updateHistory(err))
		}
	case elicit.UserInputMessage:
		request := m.input.ElicitRequest()
		if request == nil {
			break
		}

		response, err := elicit.ParseResponse(msg.Content, request.Options)
		if err != nil {
			cmds = append(cmds, updateHistory(fmt.Errorf("invalid elicit response: %w", err)))
			break
		}

		if err := m.smoke.SubmitElicitResponse(response); err != nil {
			cmds = append(cmds, updateHistory(err))
			break
		}

		m.input.ClearElicit()
		cmds = append(cmds, m.input.SetWaiting(true))
		cmds = append(cmds, updateHistory(elicit.UserResponseMessage{Response: response}))
	case elicit.UserCanceledMessage:
		if err := m.smoke.CancelElicit(); err != nil {
			cmds = append(cmds, updateHistory(err))
			break
		}

		m.input.ClearElicit()
		cmds = append(cmds, m.input.SetWaiting(true))
		cmds = append(cmds, updateHistory(msg))
	}

	return tea.Batch(cmds...)
}

func (m *Model) handleAssistantResponse(response smoke.AssistantResponseMessage) tea.Cmd {
	commands := []tea.Cmd{
		m.input.SetWaiting(false),
	}

	if response.Err != nil {
		commands = append(commands, updateHistory(response.Err))
	} else {
		commands = append(commands, updateHistory(response.Message))
	}

	return tea.Batch(commands...)
}

func (m *Model) handleAssistantUpdatedStream(response smoke.AssistantUpdatedStreamMessage) tea.Cmd {
	return updateHistory(response.Message)
}

func (m *Model) handleToolCallResponse(response smoke.ToolCallResponseMessage) tea.Cmd {
	commands := []tea.Cmd{}

	if response.Err != nil {
		commands = append(commands, updateHistory(response.Err))
	} else {
		for _, message := range response.Messages {
			commands = append(commands, updateHistory(message))
		}
	}

	return tea.Batch(commands...)
}

// Handle messages from prompt command handlers.
func (m *Model) handleCommandMessage(msg commands.Message) tea.Cmd {
	cmds := []tea.Cmd{}

	switch msg := msg.(type) {
	// This is the message we receive from the input model to execute a prompt command.
	case commands.PromptMessage:
		cmd, err := m.smoke.HandleCommand(msg)
		if err != nil {
			cmds = append(cmds, updateHistory(err))
		} else if cmd != nil {
			cmds = append(cmds, cmd)
		}

	case commands.HistoryUpdateMessage:
		cmds = append(cmds, updateHistory(msg))

	case commands.SessionUpdateMessage:
		if err := m.smoke.SetSession(msg.Session); err != nil {
			cmds = append(cmds, updateHistory(fmt.Errorf("failed to update session: %w", err)))
			break
		}

		cmds = append(cmds, updateHistory(msg))

		if msg.ResetHistory {
			newLog := make([]any, len(msg.Session.Messages))

			for i, msg := range msg.Session.Messages {
				newLog[i] = msg
			}

			resetHistory := func() tea.Msg {
				return history.ContentRefresh{
					Log: newLog,
				}
			}

			cmds = append(cmds, resetHistory)
		}

	case mode.Message:
		if err := m.smoke.SetMode(msg.Mode); err != nil {
			cmds = append(cmds, updateHistory(err))
			break
		}

		cmds = append(cmds, updateHistory(msg))

	case edit.RequestMessage:
		slog.Debug("got request to open temp file in editor", "file_path", msg.Path, "description", msg.Description, "editor", msg.Editor)

		execCmd := exec.CommandContext(context.TODO(), msg.Editor, msg.Path) //nolint:gosec // Already sanitized
		teaCmd := tea.ExecProcess(execCmd, func(err error) tea.Msg {
			return edit.ResultMessage{
				RequestMessage: msg,
				Err:            err,
			}
		})

		cmds = append(cmds, teaCmd)

	case edit.ResultMessage:
		if msg.Err != nil {
			cmds = append(cmds, updateHistory(fmt.Errorf("edit failed: %w", msg.Err)))
		} else {
			msg := commands.HistoryUpdateMessage{
				PromptMessage: msg.PromptMessage,
				Message:       "Opened file " + msg.Path + " with " + msg.Editor,
			}

			cmds = append(cmds, updateHistory(msg))
		}

	case summarize.SessionSummarizeMessage:
		cmd, err := m.smoke.HandleSummarizeMessage(msg)
		if err != nil {
			cmds = append(cmds, updateHistory(err))
		} else {
			cmds = append(cmds, cmd)
		}

	case rank.RequestMessage:
		cmd, err := m.smoke.HandleRankRequestMessage(msg)
		if err != nil {
			cmds = append(cmds, updateHistory(err))
		} else {
			cmds = append(cmds, cmd)
		}
	}

	return tea.Batch(cmds...)
}

// updateHistory is a helper function that takes any item and returns a tea.Cmd that will add it to the history
// viewport.
func updateHistory(msg any) tea.Cmd {
	// Convert regular errors to *uimsg.Error
	if err, ok := msg.(error); ok {
		if _, ok := msg.(*uimsg.Error); !ok {
			msg = uimsg.ToError(err)
		}
	}

	return func() tea.Msg {
		return history.ContentUpdate{
			Message: msg,
		}
	}
}
