// Package edit contains a prompt command that opens all or part of the conversation history in $EDITOR
package edit

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/cneill/smoke/internal/uimsg"
	"github.com/cneill/smoke/pkg/commands"
	"github.com/cneill/smoke/pkg/llms"
)

const (
	Name = "edit"

	editLast = "last"
	editAll  = "all"
)

// RequestMessage asks the UI to open a given file path in an editor, suspending the TUI.
type RequestMessage struct {
	commands.MessageType

	PromptMessage commands.PromptMessage
	Target        string
	Path          string
	Editor        string
	Description   string
}

// ResultMessage reports the result of trying to open the editor for a given path.
type ResultMessage struct {
	RequestMessage

	Err error
}

type Edit struct {
	PromptMessage commands.PromptMessage
	Target        string
}

func New(msg commands.PromptMessage) (commands.Command, error) {
	handler := &Edit{
		PromptMessage: msg,
		Target:        editAll,
	}

	if len(msg.Args) > 0 {
		switch msg.Args[0] {
		case editLast, editAll:
			handler.Target = msg.Args[0]
		default:
			return nil, fmt.Errorf("unknown edit target %q, must specify %q or %q", msg.Args[0], editLast, editAll)
		}
	}

	return handler, nil
}

func (e *Edit) Name() string { return Name }

func (e *Edit) Run(session *llms.Session) (tea.Cmd, error) {
	var content []byte

	switch e.Target {
	case editLast:
		last := session.LastByRole(llms.RoleAssistant)
		if last == nil {
			return nil, fmt.Errorf("no assistant message found to edit")
		}

		content = []byte(last.ToMarkdown())

	case editAll:
		buf := &bytes.Buffer{}
		for _, msg := range session.Messages {
			buf.WriteString(msg.ToMarkdown())
		}

		content = buf.Bytes()
	}

	tmpFile, err := os.CreateTemp("", session.Name+"_*.md")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp file: %w", err)
	}
	defer tmpFile.Close()

	if _, err := tmpFile.Write(content); err != nil {
		return nil, fmt.Errorf("failed to write temp file: %w", err)
	}

	path := tmpFile.Name()

	editor := "nvim"
	if envEditor := os.Getenv("EDITOR"); envEditor != "" {
		editor = envEditor
	}

	if _, err := exec.LookPath(editor); err != nil {
		return nil, fmt.Errorf("failed to find editor %q: %w", editor, err)
	}

	req := RequestMessage{
		PromptMessage: e.PromptMessage,
		Path:          path,
		Editor:        editor,
		Description:   "last assistant message",
	}

	return uimsg.MsgToCmd(req), nil
}
