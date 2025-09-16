package commands

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/cneill/smoke/pkg/llms"
	"github.com/cneill/smoke/pkg/prompts"
	"github.com/cneill/smoke/pkg/tools"
)

type BaseHandler struct {
	promptCommand PromptCommandMessage
}

// CommandEdit opens the last assistant message in $EDITOR
const (
	CommandEdit = "edit"

	editLast = "last"
	editAll  = "all"
)

type EditHandler struct {
	*BaseHandler

	Target string
}

func NewEditHandler(msg PromptCommandMessage) (Command, error) {
	handler := &EditHandler{
		BaseHandler: &BaseHandler{promptCommand: msg},
		Target:      editAll,
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

func (e *EditHandler) Run(session *llms.Session) (tea.Cmd, error) {
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

	req := EditRequestMessage{
		PromptCommand: e.promptCommand,
		Path:          path,
		Editor:        editor,
		Description:   "last assistant message",
	}

	return req.Cmd(), nil
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
	if len(msg.Args) == 0 {
		return nil, fmt.Errorf("%w: missing path", ErrArguments)
	}

	handler := &LoadHandler{
		BaseHandler: &BaseHandler{promptCommand: msg},
		Path:        msg.Args[0],
	}

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
		ResetHistory:  true,
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
	var sessionMessage, historyMessage string

	if p.Enabled {
		sessionMessage = prompts.PlanningOn
		historyMessage = "Entering planning mode."
	} else {
		sessionMessage = prompts.PlanningOff
		historyMessage = "Exiting planning mode."
	}

	msg := llms.SimpleMessage(llms.RoleUser, sessionMessage)

	if err := session.AddMessage(msg); err != nil {
		return nil, fmt.Errorf("failed to add plan message: %w", err)
	}

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
	if len(msg.Args) < 2 {
		return nil, fmt.Errorf("must supply tool name and arguments as JSON string")
	}

	handler := &RunHandler{
		BaseHandler: &BaseHandler{
			promptCommand: msg,
		},
		ToolName: msg.Args[0],
		RawArgs:  strings.Join(msg.Args[1:], " "),
	}

	return handler, nil
}

func (r *RunHandler) Run(session *llms.Session) (tea.Cmd, error) {
	args, err := session.Tools.GetArgs(r.ToolName, []byte(r.RawArgs))
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrArguments, err)
	}

	output, err := session.Tools.CallTool(context.TODO(), r.ToolName, args)
	if err != nil {
		return nil, fmt.Errorf("error running tool from prompt: %w", err)
	}

	msg := llms.SimpleMessage(llms.RoleUser, output)
	if err := session.AddMessage(msg); err != nil {
		return nil, fmt.Errorf("failed to add run message: %w", err)
	}

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

// CommandSession allows the user to modify the current session:
// - with the argument "new", it will start a new session without wiping the visible history
// - with the argument "clear", it will start a new session and wipe the visible history
const (
	CommandSession = "session"

	sessionClear = "clear"
)

type SessionHandler struct {
	*BaseHandler

	Command string
}

func NewSessionHandler(msg PromptCommandMessage) (Command, error) {
	if len(msg.Args) < 1 || (msg.Args[0] != "new" && msg.Args[0] != sessionClear) {
		return nil, fmt.Errorf("must supply either 'new' or 'clear' argument")
	}

	handler := &SessionHandler{
		BaseHandler: &BaseHandler{promptCommand: msg},
		Command:     msg.Args[0],
	}

	return handler, nil
}

func (s *SessionHandler) Run(session *llms.Session) (tea.Cmd, error) {
	newSession, err := llms.NewSession(&llms.SessionOpts{
		Name:          session.Name,
		SystemMessage: session.SystemMessage,
		Tools:         session.Tools,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create new session: %w", err)
	}

	msg := "Started new LLM session"
	if s.Command == sessionClear {
		msg += " and cleared history."
	} else {
		msg += " and preserved history."
	}

	update := SessionUpdateMessage{
		PromptCommand: s.promptCommand,
		Session:       newSession,
		Message:       msg,
		ResetHistory:  s.Command == sessionClear,
	}

	return update.Cmd(), nil
}

// CommandInfo displays information about the current session.
const CommandInfo = "info"

type InfoHandler struct {
	*BaseHandler
}

func NewInfoHandler(msg PromptCommandMessage) (Command, error) {
	handler := &InfoHandler{
		&BaseHandler{
			promptCommand: msg,
		},
	}

	return handler, nil
}

func (i *InfoHandler) Run(session *llms.Session) (tea.Cmd, error) {
	name := session.Name
	messageCount := session.MessageCount()
	inputTokens, outputTokens := session.Usage()
	totalTokens := inputTokens + outputTokens
	duration := time.Since(session.CreatedAt)
	toolNames := session.Tools.GetTools().Names()

	info := "**Session name:** " + name + "\n\n"
	info += fmt.Sprintf("**Messages:** user %d, assistant %d, tool call %d\n\n",
		messageCount.UserMessages, messageCount.UserMessages, messageCount.UserMessages)
	info += fmt.Sprintf("**Tokens:** input %d, output %d, total %d\n\n", inputTokens, outputTokens, totalTokens)
	info += fmt.Sprintf("**Duration:** %s\n\n", duration)
	info += fmt.Sprintf("**Tools available:** %s\n\n", strings.Join(toolNames, ", "))
	info += fmt.Sprintf("\n**System message:**\n```json\n%s\n```\n\n", session.SystemMessage)

	update := HistoryUpdateMessage{
		PromptCommand: i.promptCommand,
		Message:       info,
	}

	return update.Cmd(), nil
}

// CommandReview asks the model to review the code referenced by the user for red flags
const CommandReview = "review"

type ReviewHandler struct {
	*BaseHandler

	Enabled bool
}

func NewReviewHandler(msg PromptCommandMessage) (Command, error) {
	handler := &ReviewHandler{
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

func (r *ReviewHandler) Run(session *llms.Session) (tea.Cmd, error) {
	var systemMessage, historyMessage string

	if r.Enabled {
		systemMessage = prompts.ReviewJSON()
		historyMessage = "Entering review mode."
	} else {
		systemMessage = prompts.SystemJSON()
		historyMessage = "Exiting review mode."
	}

	session.SetSystemMessage(systemMessage)

	update := ReviewModeMessage{
		PromptCommand: r.promptCommand,
		Enabled:       r.Enabled,
		Message:       historyMessage,
		Session:       session,
	}

	return update.Cmd(), nil
}

// CommandSummarize summarizes the session history and writes it to a JSON file in the format that can be loaded as a
// session.
const (
	CommandSummarize = "summarize"

	summarizeEntire = "entire"
	summarizeFirst  = "first"
	summarizeLast   = "last"
	summarizeBefore = "before"
	summarizeAfter  = "after"
)

type SummarizeHandler struct {
	*BaseHandler

	Scope     string
	Value     string
	ValueInt  int
	ValueTime time.Time
}

func NewSummarizeHandler(msg PromptCommandMessage) (Command, error) {
	handler := &SummarizeHandler{
		BaseHandler: &BaseHandler{promptCommand: msg},
		Scope:       summarizeEntire,
	}

	if len(msg.Args) == 0 {
		return handler, nil
	}

	if len(msg.Args) != 2 {
		return nil, fmt.Errorf("usage: /summarize [--first N | --last N | --before TIME | --after TIME], e.g. --last 10")
	}

	flag := strings.TrimPrefix(msg.Args[0], "--")
	switch flag {
	case summarizeFirst, summarizeLast, summarizeBefore, summarizeAfter:
		handler.Scope = flag
	default:
		return nil, fmt.Errorf("unknown scope flag %q, use --first, --last, --before, --after", msg.Args[0])
	}

	handler.Value = msg.Args[1]

	if flag == summarizeLast || flag == summarizeFirst {
		num, err := strconv.Atoi(msg.Args[1])
		if err != nil || num < 1 {
			return nil, fmt.Errorf("value for --%s must be a positive integer", flag)
		}

		handler.ValueInt = num
	} else {
		// TODO: more flexible time format parsing
		t, err := time.Parse(time.RFC3339, msg.Args[1])
		if err != nil {
			return nil, fmt.Errorf("invalid time format for --%s, use RFC3339 (e.g., 2023-01-01T00:00:00Z)", flag)
		}

		handler.ValueTime = t
	}

	return handler, nil
}

func (s *SummarizeHandler) Run(session *llms.Session) (tea.Cmd, error) {
	filtered := s.filterMessages(session.Messages)
	sessionName := session.Name + "_summary"

	managerOpts := &tools.ManagerOpts{
		ProjectPath:      session.Tools.ProjectPath,
		SessionName:      sessionName,
		ToolInitializers: []tools.Initializer{},
		WithPlanManager:  false,
	}

	toolManager, err := tools.NewManager(managerOpts)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize tools manager: %w", err)
	}

	newSession := &llms.Session{
		Name:          sessionName,
		SystemMessage: session.SystemMessage,
		Messages:      filtered,
		CreatedAt:     time.Now(),
		Tools:         toolManager,
	}

	userMsg := llms.SimpleMessage(llms.RoleUser, prompts.Summarize)
	if err := newSession.AddMessage(userMsg); err != nil {
		return nil, fmt.Errorf("failed to add summarization prompt: %w", err)
	}

	send := SendSessionMessage{
		PromptCommand:    s.promptCommand,
		OriginalMessages: filtered,
		Session:          newSession,
	}

	return send.Cmd(), nil
}

func (s *SummarizeHandler) filterMessages(messages []*llms.Message) []*llms.Message { //nolint:cyclop
	if s.Scope == summarizeEntire || s.ValueInt >= len(messages) {
		return messages
	}

	selected := []*llms.Message{}

	switch s.Scope {
	case summarizeFirst:
		selected = messages[:s.ValueInt]
	case summarizeLast:
		selected = messages[len(messages)-s.ValueInt:]
	case summarizeBefore:
		for _, msg := range messages {
			if msg.Added.Before(s.ValueTime) {
				selected = append(selected, msg)
			}
		}
	case summarizeAfter:
		for _, msg := range messages {
			if msg.Added.After(s.ValueTime) {
				selected = append(selected, msg)
			}
		}
	}

	filtered := []*llms.Message{}

	for _, msg := range selected {
		if msg.Role == llms.RoleSystem {
			continue
		}

		filtered = append(filtered, msg)
	}

	return filtered
}
