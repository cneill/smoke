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
)

const (
	Name = "summarize"

	summarizeEntire = "entire"
	summarizeFirst  = "first"
	summarizeLast   = "last"
	summarizeBefore = "before"
	summarizeAfter  = "after"
)

type SessionSummarizeMessage struct {
	commands.MessageType

	PromptMessage    commands.PromptMessage
	OriginalMessages []*llms.Message
}

type Summarize struct {
	PromptMessage commands.PromptMessage
	Scope         string
	Value         string
	ValueInt      int
	ValueTime     time.Time
}

func New(msg commands.PromptMessage) (commands.Command, error) {
	// Handle help generation separately
	if len(msg.Args) == 1 && msg.Args[0] == "help" {
		return &Summarize{PromptMessage: msg}, nil
	}

	handler := &Summarize{
		PromptMessage: msg,
		Scope:         summarizeEntire,
	}

	if len(msg.Args) == 0 {
		return handler, nil
	}

	if len(msg.Args) != 2 {
		return nil, fmt.Errorf("%w: usage: %s", commands.ErrArguments, handler.Usage())
	}

	flag := strings.TrimPrefix(msg.Args[0], "--")
	switch flag {
	case summarizeFirst, summarizeLast, summarizeBefore, summarizeAfter:
		handler.Scope = flag
	default:
		return nil, fmt.Errorf("%w: unknown scope flag %q, use --first, --last, --before, --after", commands.ErrArguments, msg.Args[0])
	}

	handler.Value = msg.Args[1]

	if flag == summarizeLast || flag == summarizeFirst {
		num, err := strconv.Atoi(msg.Args[1])
		if err != nil || num < 1 {
			return nil, fmt.Errorf("%w: value for --%s must be a positive integer", commands.ErrArguments, flag)
		}

		handler.ValueInt = num
	} else {
		// TODO: more flexible time format parsing
		t, err := time.Parse(time.RFC3339, msg.Args[1])
		if err != nil {
			return nil, fmt.Errorf("%w: invalid time format for --%s, use RFC3339 (e.g., 2023-01-01T00:00:00Z)", commands.ErrArguments, flag)
		}

		handler.ValueTime = t
	}

	return handler, nil
}

func (s *Summarize) Name() string { return Name }

func (s *Summarize) Help() string {
	return "Asks the LLM to summarize part or all of the message history."
}

func (s *Summarize) Usage() string {
	return "/summarize [--first N | --last N | --before TIME | --after TIME]"
}

func (s *Summarize) Run(session *llms.Session) (tea.Cmd, error) {
	// TODO: make a read-only view of messages...?
	filtered := s.filterMessages(session.Messages)
	msg := SessionSummarizeMessage{
		PromptMessage:    s.PromptMessage,
		OriginalMessages: filtered,
	}

	return uimsg.MsgToCmd(msg), nil
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
