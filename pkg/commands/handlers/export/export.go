// Package export contains a prompt command that exports the current session to a Markdown file for human reading.
package export

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/cneill/smoke/internal/uimsg"
	"github.com/cneill/smoke/pkg/commands"
	"github.com/cneill/smoke/pkg/llms"
)

const Name = "export"

type Export struct{}

func New() (commands.Command, error) {
	return &Export{}, nil
}

func (e *Export) Name() string { return Name }

func (e *Export) Help() string {
	return "Exports the current session to a Markdown file."
}

func (e *Export) Usage() string {
	return "export [path]"
}

func (e *Export) Run(_ context.Context, msg commands.PromptMessage, session *llms.Session) (tea.Cmd, error) {
	path := fmt.Sprintf("%s_%s.md", session.Name, time.Now().Format(time.DateTime))

	if len(msg.Args) > 0 {
		path = msg.Args[0]
	}

	buf := &bytes.Buffer{}
	for _, msg := range session.Messages {
		buf.WriteString(msg.ToMarkdown())
	}

	slog.Debug("exporting session to file as markdown", "path", path, "num_messages", len(session.Messages))

	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		return nil, fmt.Errorf("failed to export session to file %q: %w", path, err)
	}

	content := &uimsg.HistoryContent{
		Blocks: []uimsg.HistoryBlock{
			{
				Type:  uimsg.HistoryBlockFields,
				Title: "Export complete",
				Fields: []uimsg.HistoryField{
					uimsg.NewField("Path", path),
					uimsg.NewField("Format", "Markdown"),
					uimsg.NewField("Messages", strconv.Itoa(len(session.Messages))),
				},
			},
		},
	}

	update := commands.HistoryUpdateMessage{
		PromptMessage: msg,
		Content:       content,
	}

	return uimsg.MsgToCmd(update), nil
}
