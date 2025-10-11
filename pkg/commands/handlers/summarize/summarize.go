// Package summarize contains a prompt command that asks the LLM to summarize all or a subset of the message history and
// causes the history to be updated in the UI
package summarize

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/cneill/smoke/internal/uimsg"
	"github.com/cneill/smoke/pkg/commands"
	"github.com/cneill/smoke/pkg/llms"
	"github.com/cneill/smoke/pkg/prompts"
	"github.com/cneill/smoke/pkg/tools"
)

const (
	Name = "summarize"

	summarizeEntire = "entire"
	summarizeFirst  = "first"
	summarizeLast   = "last"
	summarizeBefore = "before"
	summarizeAfter  = "after"
)

type Summarize struct {
	PromptMessage commands.PromptMessage
	Scope         string
	Value         string
	ValueInt      int
	ValueTime     time.Time
}

func New(msg commands.PromptMessage) (commands.Command, error) {
	handler := &Summarize{
		PromptMessage: msg,
		Scope:         summarizeEntire,
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

func (s *Summarize) Name() string { return Name }

func (s *Summarize) Run(session *llms.Session) (tea.Cmd, error) {
	// TODO: make a read-only view of messages...
	filtered := s.filterMessages(session.Messages)
	sessionName := session.Name + "_summary"
	systemMessage := prompts.SummarizeSystemPrompt(filtered...).Markdown()

	managerOpts := &tools.ManagerOpts{
		ProjectPath:      session.Tools.ProjectPath,
		SessionName:      sessionName,
		ToolInitializers: tools.SummarizeTools(),
		WithPlanManager:  true,
	}

	toolManager, err := tools.NewManager(managerOpts)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize tools manager for summarization: %w", err)
	}

	newSession, err := llms.NewSession(&llms.SessionOpts{
		Name:            sessionName,
		SystemMessage:   systemMessage,
		SystemAsMessage: session.SystemAsMessage,
		Tools:           toolManager,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to initialize new session for summarization: %w", err)
	}

	userMessage := llms.SimpleMessage(llms.RoleUser, "Please proceed to summarizing the provided messages.")
	if err := newSession.AddMessage(userMessage); err != nil {
		return nil, fmt.Errorf("failed to add user summarization message to summarization session: %w", err)
	}

	send := commands.SendSessionMessage{
		PromptMessage:    s.PromptMessage,
		OriginalMessages: filtered,
		Session:          newSession,
	}

	return uimsg.MsgToCmd(send), nil
}

func (s *Summarize) filterMessages(messages []*llms.Message) []*llms.Message { //nolint:cyclop
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
