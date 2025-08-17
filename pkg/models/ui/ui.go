// Package ui contains a Bubbletea model that wraps other models like [history.Model] and [input.Model], as well as the
// [*smoke.Smoke] struct that contains and modifies application state. It is the main model for the application,
// executed as part of the main() function.
package ui

import (
	"context"
	"fmt"
	"log/slog"
	"os"

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

	// don't send key messages unless the input is unfocused or waiting
	if _, ok := msg.(tea.KeyMsg); !ok || ok && (!m.input.Focused() || m.input.Waiting()) {
		historyModel, historyCmd := m.history.Update(msg)
		m.history = historyModel

		if historyCmd != nil {
			cmds = append(cmds, historyCmd)
		}
	}

	switch msg := msg.(type) {
	case tea.WindowSizeMsg, input.ResizeMessage:
		m.resize(msg)
	case tea.KeyMsg:
		switch msg.Type { //nolint:exhaustive,gocritic
		case tea.KeyCtrlC:
			return m, tea.Quit
		}
	case input.UserMessage:
		if cmd := m.handleUserMessage(msg); cmd != nil {
			cmds = append(cmds, cmd)
		}
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
		m.smoke.SetSession(msg.Session)

		newLog := []any{}

		for _, msg := range msg.Session.Messages {
			newLog = append(newLog, msg)
		}

		refresh := func() tea.Msg {
			return history.ContentRefresh{
				Log: newLog,
			}
		}

		cmds = append(cmds, tea.Batch(refresh, updateHistory(msg)))
	case commands.PlanningModeMessage:
		m.smoke.SetPlanningMode(msg.Enabled)
		cmds = append(cmds, updateHistory(msg.SessionMessage))
		cmds = append(cmds, updateHistory(msg))
	case assistantError:
		cmds = append(cmds, updateHistory(msg.err))
	case assistantResponse:
		if cmd := m.handleAssistantResponse(msg); cmd != nil {
			cmds = append(cmds, cmd)
		}
	case toolCallResponse:
		if cmd := m.handleToolCallResponse(msg); cmd != nil {
			cmds = append(cmds, cmd)
		}
	}

	return m, tea.Batch(cmds...)
}

func (m *Model) View() string {
	return fmt.Sprintf("%s%s%s", m.history.View(), gap, m.input.View())
}

type assistantResponse struct {
	message *llms.Message
}

type assistantError struct {
	err error
}

func (m *Model) resize(msg tea.Msg) {
	lineHeight := m.input.LineHeight()

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.history.Resize(msg.Width, msg.Height-(lineHeight+1)) // +1 for the border
		m.input.Resize(msg.Width, lineHeight)
	case input.ResizeMessage:
		delta := lineHeight - m.input.GetHeight() // how many lines did we resize by
		width := m.history.GetWidth()
		m.history.Resize(width, m.history.GetHeight()-delta)
		m.input.Resize(width, lineHeight)
	}
}

func (m *Model) handleUserMessage(msg input.UserMessage) tea.Cmd {
	llmMessage := llms.SimpleMessage(llms.RoleUser, msg.Content)

	sendMessage := func() tea.Msg {
		slog.Debug("got user message", "msg", llmMessage)

		// TODO: reasonable, adjustable context timeouts
		// ctx, cancel := context.WithTimeout(context.Background(), time.Second*60)
		// defer cancel()

		response, err := m.smoke.SendUserMessage(context.TODO(), llmMessage)
		if err != nil {
			return assistantError{err}
		}

		return assistantResponse{response}
	}

	return tea.Batch(updateHistory(llmMessage), sendMessage, m.input.SetWaiting(true))
}

type toolCallResponse struct {
	messages []*llms.Message
	err      error
}

func (m *Model) handleAssistantResponse(response assistantResponse) tea.Cmd {
	commands := []tea.Cmd{
		updateHistory(response.message),
	}

	if response.message.HasToolCalls() {
		commands = append(commands, func() tea.Msg {
			slog.Debug("got assistant message", "msg", response.message)

			results, err := m.smoke.HandleAssistantToolCalls(response.message)
			if err != nil {
				return toolCallResponse{err: err}
			}

			return toolCallResponse{messages: results}
		})
	} else {
		m.input.SetWaiting(false)
	}

	return tea.Batch(commands...)
}

func (m *Model) handleToolCallResponse(response toolCallResponse) tea.Cmd {
	commands := []tea.Cmd{}

	if response.err != nil {
		commands = append(commands, updateHistory(response.err))
	}

	if response.messages != nil {
		for _, message := range response.messages {
			commands = append(commands, updateHistory(message))
		}

		commands = append(commands, func() tea.Msg {
			// TODO: fix the logging for a slice of these messages?
			slog.Debug("got tool call results", "messages", response.messages)

			response, err := m.smoke.HandleToolCallResults(context.TODO(), response.messages)
			if err != nil {
				commands = append(commands, m.input.SetWaiting(false))
				return assistantError{err}
			}

			return assistantResponse{response}
		})
	}

	return tea.Batch(commands...)
}

func updateHistory(msg any) tea.Cmd {
	return func() tea.Msg {
		return history.ContentUpdate{
			Message: msg,
		}
	}
}
