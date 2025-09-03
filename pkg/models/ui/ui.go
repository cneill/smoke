// Package ui contains a Bubbletea model that wraps other models like [history.Model] and [input.Model], as well as the
// [*smoke.Smoke] struct that contains and modifies application state. It is the main model for the application,
// executed as part of the main() function.
package ui

import (
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"time"

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

	chunkChan chan (*llms.Message)
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
	case input.CancelUserMessage:
		m.smoke.CancelUserMessage(msg.Err)
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
		m.smoke.SetPlanningMode(msg.Enabled)
		cmds = append(cmds, updateHistory(msg.SessionMessage))
		cmds = append(cmds, updateHistory(msg))
	case commands.EditRequestMessage:
		slog.Debug("got request to open temp file in editor", "file_path", msg.Path, "description", msg.Description, "editor", msg.Editor)

		execCmd := exec.Command(msg.Editor, msg.Path)
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
	case commands.SendSessionMessage:
		if cmd := m.smoke.SendCommandMessage(msg); cmd != nil {
			cmds = append(cmds, cmd)
		}
	case smoke.AssistantResponseMessage:
		if msg.Err != nil {
			cmds = append(cmds, updateHistory(msg.Err))
		} else if cmd := m.handleAssistantResponse(msg); cmd != nil {
			cmds = append(cmds, cmd)
		}
	case smoke.ToolCallResponseMessage:
		if msg.Err != nil {
			cmds = append(cmds, updateHistory(msg.Err))
		} else if cmd := m.handleToolCallResponse(msg); cmd != nil {
			cmds = append(cmds, cmd)
		}
	case smoke.SendCommandMessageResponseMessage:
		if cmd := m.handleSendCommandMessage(msg); cmd != nil {
			cmds = append(cmds, cmd)
		}
	}

	return m, tea.Batch(cmds...)
}

func (m *Model) View() string {
	return fmt.Sprintf("%s%s%s", m.history.View(), gap, m.input.View())
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

func (m *Model) chunkListener() tea.Cmd {
	return func() tea.Msg {
		if m.chunkChan == nil {
			return nil
			// return m.input.SetWaiting(false)
		}

		// TODO: make this configurable? remove?
		timer := time.NewTimer(time.Second * 60)
		select {
		case msg, ok := <-m.chunkChan:
			if !ok {
				m.chunkChan = nil
				return nil
				// return m.input.SetWaiting(false)
			}

			return smoke.AssistantResponseMessage{
				Message: msg,
			}
		case <-timer.C:
			// return tea.Batch(updateHistory(fmt.Errorf("timed out waiting for next chunk")), m.input.SetWaiting(false))
			return updateHistory(fmt.Errorf("timed out waiting for next chunk"))
		}
	}
}

func (m *Model) handleUserMessage(msg input.UserMessage) tea.Cmd {
	llmMessage := llms.SimpleMessage(llms.RoleUser, msg.Content)
	commands := []tea.Cmd{
		updateHistory(llmMessage),
		m.input.SetWaiting(true),
	}

	slog.Debug("got user message", "message", llmMessage)

	// var sendMessage tea.Cmd

	if m.smoke.ShouldStream() {
		m.chunkChan = make(chan *llms.Message)

		cmd, err := m.smoke.SendUserMessageStreaming(llmMessage, m.chunkChan)
		if err != nil {
			return updateHistory(err)
		}

		commands = append(commands, cmd)
		commands = append(commands, m.chunkListener())
	} else {
		cmd, err := m.smoke.SendUserMessage(llmMessage)
		if err != nil {
			return updateHistory(err)
		}

		if cmd != nil {
			commands = append(commands, cmd)
		}
	}

	return tea.Batch(commands...)
}

func (m *Model) handleAssistantResponse(response smoke.AssistantResponseMessage) tea.Cmd {
	commands := []tea.Cmd{
		updateHistory(response.Message),
	}

	slog.Debug("got assistant message", "message", response.Message)

	// update the usage based on the latest response
	m.input.UpdateUsage(m.smoke.GetUsage())

	switch {
	case response.Message.IsStreamed && !response.Message.IsFinalized:
		commands = append(commands, m.chunkListener())
	case (!response.Message.IsStreamed || response.Message.IsFinalized) && !response.Message.HasToolCalls():
		m.input.SetWaiting(false)
	}

	if response.Message.HasToolCalls() && (!response.Message.IsStreamed || response.Message.IsFinalized) {
		cmd, err := m.smoke.HandleAssistantToolCalls(response.Message)
		if err != nil {
			m.input.SetWaiting(false)
			return updateHistory(err)
		}

		commands = append(commands, cmd)
	}

	return tea.Batch(commands...)
}

func (m *Model) handleToolCallResponse(response smoke.ToolCallResponseMessage) tea.Cmd {
	commands := []tea.Cmd{}

	for i, msg := range response.Messages {
		slog.Debug("got tool call response", "message_num", i, "message", msg)
	}

	if response.Err != nil {
		commands = append(commands, updateHistory(response.Err))
	} else if response.Messages != nil {
		for _, message := range response.Messages {
			commands = append(commands, updateHistory(message))
		}

		cmd, err := m.smoke.HandleToolCallResults(response.Messages)
		if err != nil {
			return updateHistory(err)
		}

		if cmd != nil {
			commands = append(commands, cmd)
		}
	}

	return tea.Batch(commands...)
}

func (m *Model) handleSendCommandMessage(msg smoke.SendCommandMessageResponseMessage) tea.Cmd {
	if msg.Err != nil {
		return updateHistory(fmt.Errorf("error sending LLM message from command: %w", msg.Err))
	}

	return m.smoke.HandleCommandMessageResponse(msg)
}

func updateHistory(msg any) tea.Cmd {
	return func() tea.Msg {
		return history.ContentUpdate{
			Message: msg,
		}
	}
}
