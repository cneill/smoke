package commands

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/cneill/smoke/pkg/llms"
	"github.com/cneill/smoke/pkg/tools"
)

type BaseHandler struct {
	promptCommand PromptCommandMessage
}

// CommandClear clears the current session contents.
const CommandClear = "clear"

type ClearHandler struct {
	*BaseHandler
}

func NewClearHandler(msg PromptCommandMessage) (Command, error) {
	handler := &ClearHandler{
		BaseHandler: &BaseHandler{promptCommand: msg},
	}

	return handler, nil
}

func (c *ClearHandler) Run(session *llms.Session) (tea.Cmd, error) {
	newSession, err := llms.NewSession(&llms.SessionOpts{
		Name:          session.Name,
		SystemMessage: session.SystemMessage,
		Tools:         session.Tools,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to clear session and create new one: %w", err)
	}

	update := SessionUpdateMessage{
		PromptCommand: c.promptCommand,
		Session:       newSession,
		Message:       "Cleared session.",
	}

	return update.Cmd(), nil
}

// CommandExit closes the program.
const CommandExit = "exit"

type ExitHandler struct {
	*BaseHandler
}

func NewExitHandler(msg PromptCommandMessage) (Command, error) {
	handler := &ExitHandler{
		&BaseHandler{
			promptCommand: msg,
		},
	}

	return handler, nil
}

func (e *ExitHandler) Run(_ *llms.Session) (tea.Cmd, error) {
	return tea.Quit, nil
}

// CommandExport saves the current session to a file in JSON format that can be used with the 'load' command.
const CommandExport = "export"

type ExportHandler struct {
	*BaseHandler

	Path string
}

func NewExportHandler(msg PromptCommandMessage) (Command, error) {
	handler := &ExportHandler{
		BaseHandler: &BaseHandler{
			promptCommand: msg,
		},
	}
	if len(msg.Args) > 0 {
		handler.Path = msg.Args[0]
	}

	return handler, nil
}

func (e *ExportHandler) Run(session *llms.Session) (tea.Cmd, error) {
	if e.Path == "" {
		e.Path = fmt.Sprintf("%s_%s.json", session.Name, time.Now().Format(time.DateTime))
	}

	sessionBytes, err := json.MarshalIndent(session, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal session JSON: %w", err)
	}

	slog.Debug("exporting session to file", "path", e.Path, "num_messages", len(session.Messages))

	if err := os.WriteFile(e.Path, sessionBytes, 0o644); err != nil {
		return nil, fmt.Errorf("failed to export session to file %q: %w", e.Path, err)
	}

	update := HistoryUpdateMessage{
		PromptCommand: e.promptCommand,
		Message:       "Exported session to file " + e.Path + " in JSON format.",
	}

	return update.Cmd(), nil
}

// CommandLoad loads a session from a JSON file and replaces the current session with it.
const CommandLoad = "load"

type LoadHandler struct {
	*BaseHandler

	Path string
}

func NewLoadHandler(msg PromptCommandMessage) (Command, error) {
	handler := &LoadHandler{
		BaseHandler: &BaseHandler{promptCommand: msg},
	}

	if len(msg.Args) == 0 {
		return nil, fmt.Errorf("%w: missing path", ErrArguments)
	}

	handler.Path = msg.Args[0]

	return handler, nil
}

func (l *LoadHandler) Run(session *llms.Session) (tea.Cmd, error) {
	if l.Path == "" {
		return nil, fmt.Errorf("%w: must provide a path to a session file", ErrArguments)
	}

	data, err := os.ReadFile(l.Path)
	if err != nil {
		return nil, fmt.Errorf("failed to read session file %q: %w", l.Path, err)
	}

	loaded := &llms.Session{}
	if err := json.Unmarshal(data, loaded); err != nil {
		return nil, fmt.Errorf("failed to unmarshal session JSON from %q: %w", l.Path, err)
	}

	loaded.Tools = session.Tools

	slog.Debug("loaded session from file", "path", l.Path, "num_messages", len(loaded.Messages))

	update := SessionUpdateMessage{
		PromptCommand: l.promptCommand,
		Session:       loaded,
		Message:       "Loaded session from file " + l.Path + ".",
	}

	return update.Cmd(), nil
}

// CommandPlan prevents the model from making changes to files other than the plan file.
const CommandPlan = "plan"

type PlanHandler struct {
	*BaseHandler

	Enabled bool
}

func NewPlanHandler(msg PromptCommandMessage) (Command, error) {
	handler := &PlanHandler{
		BaseHandler: &BaseHandler{
			promptCommand: msg,
		},
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
			return nil, fmt.Errorf("%w: %s", ErrArguments, msg.Args[0])
		}
	}

	handler.Enabled = boolVal

	return handler, nil
}

func (p *PlanHandler) Run(session *llms.Session) (tea.Cmd, error) {
	var (
		sessionMessage string
		historyMessage string
	)

	if p.Enabled {
		sessionMessage = "!!IMPORTANT!! You are now in planning mode. Refer to `plan_process` and do not proceed to " +
			"`work_process` until you have exited planning mode."
		historyMessage = "Entering planning mode."
	} else {
		sessionMessage = "!!IMPORTANT!! You have now exited planning mode. Refer to the plan file and proceed to " +
			"`work_process` to begin your work."
		historyMessage = "Exiting planning mode."
	}

	msg := llms.SimpleMessage(llms.RoleUser, sessionMessage)

	session.AddMessage(msg)

	update := PlanningModeMessage{
		PromptCommand:  p.promptCommand,
		Enabled:        p.Enabled,
		Message:        historyMessage,
		SessionMessage: msg,
	}

	return update.Cmd(), nil
}

// CommandSave saves the current session to a Markdown file
const CommandRun = "run"

type RunHandler struct {
	*BaseHandler

	ToolName string
	RawArgs  string
}

func NewRunHandler(msg PromptCommandMessage) (Command, error) {
	handler := &RunHandler{
		BaseHandler: &BaseHandler{
			promptCommand: msg,
		},
	}

	if len(msg.Args) < 2 {
		return nil, fmt.Errorf("must supply tool name and arguments as JSON string")
	}

	handler.ToolName = msg.Args[0]
	handler.RawArgs = strings.Join(msg.Args[1:], " ")

	return handler, nil
}

func (r *RunHandler) Run(session *llms.Session) (tea.Cmd, error) {
	params, err := session.Tools.Params(r.ToolName)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrArguments, err)
	}

	args, err := tools.GetArgs([]byte(r.RawArgs), params)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrArguments, err)
	}

	output, err := session.Tools.CallTool(context.TODO(), r.ToolName, args)
	if err != nil {
		return nil, fmt.Errorf("error running tool from prompt: %w", err)
	}

	msg := llms.SimpleMessage(llms.RoleUser, output)
	session.AddMessage(msg)

	updateMsg := fmt.Sprintf("User called tool %q with args %q:\n\n%s\n", r.ToolName, r.RawArgs, output)

	update := HistoryUpdateMessage{
		PromptCommand: r.promptCommand,
		Message:       updateMsg,
	}

	return update.Cmd(), nil
}

// CommandSave saves the current session to a Markdown file
const CommandSave = "save"

type SaveHandler struct {
	*BaseHandler

	Path string
}

func NewSaveHandler(msg PromptCommandMessage) (Command, error) {
	handler := &SaveHandler{
		BaseHandler: &BaseHandler{
			promptCommand: msg,
		},
	}
	if len(msg.Args) > 0 {
		handler.Path = msg.Args[0]
	}

	return handler, nil
}

func (s *SaveHandler) Run(session *llms.Session) (tea.Cmd, error) {
	if s.Path == "" {
		s.Path = fmt.Sprintf("%s_%s.md", session.Name, time.Now().Format(time.DateTime))
	}

	buf := &bytes.Buffer{}
	for _, msg := range session.Messages {
		buf.WriteString(msg.ToMarkdown())
	}

	slog.Debug("saving session to file as markdown", "path", s.Path, "num_messages", len(session.Messages))

	if err := os.WriteFile(s.Path, buf.Bytes(), 0o644); err != nil {
		return nil, fmt.Errorf("failed to save session to file %q: %w", s.Path, err)
	}

	update := HistoryUpdateMessage{
		PromptCommand: s.promptCommand,
		Message:       "Saved session to file " + s.Path + " in Markdown format.",
	}

	return update.Cmd(), nil
}

// CommandEdit opens the last assistant message in $EDITOR
const CommandEdit = "edit"

type EditHandler struct {
	*BaseHandler

	// For now we only support "last"; future: indexes / code blocks
}

func NewEditHandler(msg PromptCommandMessage) (Command, error) {
	// For the first iteration, either no args or "last" are accepted
	if len(msg.Args) > 0 && msg.Args[0] != "last" {
		return nil, fmt.Errorf("%w: only 'last' is supported for now", ErrArguments)
	}

	return &EditHandler{BaseHandler: &BaseHandler{promptCommand: msg}}, nil
}

func (e *EditHandler) Run(session *llms.Session) (tea.Cmd, error) {
	// Find last assistant message
	last := session.LastByRole(llms.RoleAssistant)
	if last == nil {
		return nil, fmt.Errorf("no assistant message found to edit")
	}

	// Build markdown content using ToMarkdown for consistency with /save
	content := []byte(last.ToMarkdown())

	// Create temp file with .md extension
	pattern := fmt.Sprintf("%s_%s_last.md", session.Name, time.Now().Format(time.DateTime))

	tmpFile, err := os.CreateTemp("", pattern)
	if err != nil {
		return nil, fmt.Errorf("failed to create temp file: %w", err)
	}
	defer tmpFile.Close()

	if _, err := tmpFile.Write(content); err != nil {
		return nil, fmt.Errorf("failed to write temp file: %w", err)
	}

	path := tmpFile.Name()

	// Ask UI to open editor for this path; UI will suspend/resume
	req := EditRequestMessage{
		Path:        path,
		Description: "last assistant message",
	}

	return req.Cmd(), nil
}
