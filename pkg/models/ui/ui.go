package ui

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	tea "github.com/charmbracelet/bubbletea"
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
		Width:           width,
		Height:          2,
		MaxHeight:       5,
		PlaceholderText: "Enter your message...",
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
	commands := []tea.Cmd{}

	inputModel, inputCmd := m.input.Update(msg)
	m.input = inputModel

	if inputCmd != nil {
		commands = append(commands, inputCmd)
	}

	historyModel, historyCmd := m.history.Update(msg)
	m.history = historyModel

	if historyCmd != nil {
		commands = append(commands, historyCmd)
	}

	switch msg := msg.(type) {
	case tea.WindowSizeMsg, input.ResizeMessage:
		m.resize(msg)
	case tea.KeyMsg:
		switch msg.Type { //nolint:exhaustive
		case tea.KeyCtrlC:
			return m, tea.Quit
		}
	case input.UserMessage:
		if cmd := m.handleUserMessage(msg); cmd != nil {
			commands = append(commands, cmd)
		}
	case input.ExitCommand:
		return m, tea.Quit
	case input.UnknownCommand:
		slog.Warn("unknown command", "command", msg.Command, "args", msg.Args)
	case assistantError:
		commands = append(commands, updateHistory(msg.err))
	case assistantResponse:
		if cmd := m.handleAssistantResponse(msg); cmd != nil {
			commands = append(commands, cmd)
		}
	case toolCallResponse:
		if cmd := m.handleToolCallResponse(msg); cmd != nil {
			commands = append(commands, cmd)
		}
	}

	return m, tea.Batch(commands...)
}

func (m *Model) resize(msg tea.Msg) {
	lineHeight := m.input.LineHeight()

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.history.Resize(msg.Width, msg.Height-lineHeight)
		m.input.Resize(msg.Width, lineHeight)
	case input.ResizeMessage:
		delta := lineHeight - m.input.GetHeight() // how many lines did we resize by
		width := m.history.GetWidth()
		m.history.Resize(width, m.history.GetHeight()-delta)
		m.input.Resize(width, lineHeight)
	}
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

	return tea.Batch(updateHistory(llmMessage), sendMessage)
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
