// Package summarize contains a prompt command that asks the LLM to summarize all or a subset of the message history and
// causes the history to be updated in the UI
package summarize

import (
	"context"
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

type Summarize struct{}

type opts struct {
	scope     string
	value     string
	valueInt  int
	valueTime time.Time
}

func defaultOpts() *opts {
	return &opts{
		scope: summarizeEntire,
	}
}

func New() (commands.Command, error) {
	return &Summarize{}, nil
}

func (s *Summarize) Name() string { return Name }

func (s *Summarize) Help() string {
	return "Asks the LLM to summarize part or all of the message history."
}

func (s *Summarize) Usage() string {
	return "summarize [--first N | --last N | --before TIME | --after TIME]"
}

func (s *Summarize) Run(_ context.Context, msg commands.PromptMessage, session *llms.Session) (tea.Cmd, error) {
	// TODO: make this work like /rank, don't push all the logic to Smoke
	// TODO: make a read-only view of messages...?
	opts, err := s.parseOpts(msg)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", commands.ErrArguments, err)
	}

	filtered := s.filterMessages(opts, session.Messages)

	summarizeMsg := SessionSummarizeMessage{
		PromptMessage:    msg,
		OriginalMessages: filtered,
	}

	return uimsg.MsgToCmd(summarizeMsg), nil
}

func (s *Summarize) parseOpts(msg commands.PromptMessage) (*opts, error) {
	opts := defaultOpts()

	if len(msg.Args) != 2 {
		return nil, fmt.Errorf("usage: %s", s.Usage())
	}

	flag := strings.TrimPrefix(msg.Args[0], "--")
	switch flag {
	case summarizeFirst, summarizeLast, summarizeBefore, summarizeAfter:
		opts.scope = flag
	default:
		return nil, fmt.Errorf("unknown scope flag %q, use --first, --last, --before, --after", msg.Args[0])
	}

	opts.value = msg.Args[1]

	if flag == summarizeLast || flag == summarizeFirst {
		num, err := strconv.Atoi(msg.Args[1])
		if err != nil || num < 1 {
			return nil, fmt.Errorf("value for --%s must be a positive integer", flag)
		}

		opts.valueInt = num
	} else {
		// TODO: more flexible time format parsing
		t, err := time.Parse(time.RFC3339, msg.Args[1])
		if err != nil {
			return nil, fmt.Errorf("invalid time format for --%s, use RFC3339 (e.g., 2023-01-01T00:00:00Z)", flag)
		}

		opts.valueTime = t
	}

	return opts, nil
}

func (s *Summarize) filterMessages(opts *opts, messages []*llms.Message) []*llms.Message { //nolint:cyclop
	if opts.scope == summarizeEntire || opts.valueInt >= len(messages) {
		return messages
	}

	selected := []*llms.Message{}

	switch opts.scope {
	case summarizeFirst:
		selected = messages[:opts.valueInt]
	case summarizeLast:
		selected = messages[len(messages)-opts.valueInt:]
	case summarizeBefore:
		for _, msg := range messages {
			if msg.Added.Before(opts.valueTime) {
				selected = append(selected, msg)
			}
		}
	case summarizeAfter:
		for _, msg := range messages {
			if msg.Added.After(opts.valueTime) {
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
