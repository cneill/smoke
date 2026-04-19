// Package edit contains a prompt command that opens all or part of the conversation history in $EDITOR
package edit

import (
	"bytes"
	"context"
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

type Edit struct{}

func New() (commands.Command, error) {
	return &Edit{}, nil
}

func (e *Edit) Name() string { return Name }

func (e *Edit) Help() string {
	return "Opens the conversation history or the last assistant message in your editor."
}

func (e *Edit) Usage() string {
	return "edit [last|all]"
}

func (e *Edit) Run(_ context.Context, msg commands.PromptMessage, session *llms.Session) (tea.Cmd, error) {
	var (
		target  string
		content []byte
	)

	if len(msg.Args) > 0 {
		switch msg.Args[0] {
		case editLast, editAll:
			target = msg.Args[0]
		default:
			return nil, fmt.Errorf("%w: unknown edit target %q, must specify %q or %q",
				commands.ErrArguments, msg.Args[0], editLast, editAll)
		}
	}

	switch target {
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
		PromptMessage: msg,
		Path:          path,
		Editor:        editor,
		Description:   "last assistant message",
	}

	return uimsg.MsgToCmd(req), nil
}
