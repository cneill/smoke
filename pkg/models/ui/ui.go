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
	"github.com/cneill/smoke/pkg/commands"
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
	case smoke.UsageUpdateMessage:
		slog.Debug("Usage update", "input_tokens", msg.InputTokens, "output_tokens", msg.OutputTokens)
		m.input.UpdateUsage(msg.InputTokens, msg.OutputTokens)
	case smoke.ToolCallResponseMessage:
		cmds = append(cmds, m.handleToolCallResponse(msg))
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
	case commands.PromptCommandMessage:
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
			newLog := []any{}

			for _, msg := range msg.Session.Messages {
				newLog = append(newLog, msg)
			}

			resetHistory := func() tea.Msg {
				return history.ContentRefresh{
					Log: newLog,
				}
			}

			cmds = append(cmds, resetHistory)
		}

	case commands.PlanningModeMessage:
		if err := m.smoke.SetSession(msg.Session); err != nil {
			cmds = append(cmds, updateHistory(fmt.Errorf("failed to update session for planning mode switch: %w", err)))
			break
		}

		if msg.Enabled {
			m.smoke.SetMode(smoke.ModePlanning)
		} else {
			m.smoke.SetMode(smoke.ModeNormal)
		}

		cmds = append(cmds, updateHistory(msg))
		// TODO: do away with these separate mode messages, unify them with session update message?

	case commands.ReviewModeMessage:
		if err := m.smoke.SetSession(msg.Session); err != nil {
			cmds = append(cmds, updateHistory(fmt.Errorf("failed to update session for review mode switch: %w", err)))
			break
		}

		if msg.Enabled {
			m.smoke.SetMode(smoke.ModeReview)
		} else {
			m.smoke.SetMode(smoke.ModeNormal)
		}

		cmds = append(cmds, updateHistory(msg))

	case commands.EditRequestMessage:
		slog.Debug("got request to open temp file in editor", "file_path", msg.Path, "description", msg.Description, "editor", msg.Editor)

		execCmd := exec.CommandContext(context.TODO(), msg.Editor, msg.Path) //nolint:gosec // Already sanitized
		teaCmd := tea.ExecProcess(execCmd, func(err error) tea.Msg {
			return commands.EditResultMessage{
				EditRequestMessage: msg,
				Err:                err,
			}
		})

		cmds = append(cmds, teaCmd)

	case commands.EditResultMessage:
		if msg.Err != nil {
			cmds = append(cmds, updateHistory(fmt.Errorf("edit failed: %w", msg.Err)))
		} else {
			msg := commands.HistoryUpdateMessage{
				PromptCommand: msg.PromptCommand,
				Message:       "Opened file " + msg.Path + " with " + msg.Editor,
			}

			cmds = append(cmds, updateHistory(msg))
		}

		// case commands.SendSessionMessage:
		// 	if cmd := m.smoke.SendCommandMessage(msg); cmd != nil {
		// 		cmds = append(cmds, cmd)
		// 	}
	}

	return tea.Batch(cmds...)
}

// func (m *Model) handleSendCommandMessage(msg smoke.SendCommandMessageResponseMessage) tea.Cmd {
// 	if msg.Err != nil {
// 		return updateHistory(fmt.Errorf("error sending LLM message from command: %w", msg.Err))
// 	}
//
// 	return m.smoke.HandleCommandMessageResponse(msg)
// }

// updateHistory is a helper function that takes any item and returns a tea.Cmd that will add it to the history
// viewport.
func updateHistory(msg any) tea.Cmd {
	return func() tea.Msg {
		return history.ContentUpdate{
			Message: msg,
		}
	}
}
