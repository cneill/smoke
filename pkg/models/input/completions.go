package input

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

type CompletionType int

const (
	CompletionTypeNone CompletionType = iota
	CompletionTypeCommand
	CompletionTypeSkill
)

type CompletionState struct {
	commandCompleter func(string) []string
	skillCompleter   func(string) []string
	completionType   CompletionType
	userText         string
	suggestedText    string
}

func NewCompletionState(commandCompleter, skillCompleter func(string) []string) (*CompletionState, error) {
	if commandCompleter == nil {
		return nil, fmt.Errorf("must supply command completer")
	}

	if skillCompleter == nil {
		return nil, fmt.Errorf("must supply skill completer")
	}

	return &CompletionState{
		commandCompleter: commandCompleter,
		skillCompleter:   skillCompleter,
	}, nil
}

func (c *CompletionState) InCommandCompletion() bool {
	return c.completionType == CompletionTypeCommand
}

func (c *CompletionState) InSkillCompletion() bool {
	return c.completionType == CompletionTypeSkill
}

func (c *CompletionState) InCompletion() bool {
	return c.InCommandCompletion() || c.InSkillCompletion()
}

// HandleUserCompletionKey returns true if 'msg' starts, or is part of, a completion for skills/commands.
func (c *CompletionState) HandleUserCompletionKey(msg tea.KeyMsg, currentText string) bool {
	keyVal := msg.String()

	// check if we have a leading character signaling the start of a completion
	if !c.InCompletion() {
		switch {
		case keyVal == "/" && currentText == "":
			c.completionType = CompletionTypeCommand
		case keyVal == "$":
			// make sure we're not in the middle of a word
			if currentText != "" && !strings.ContainsAny(string(currentText[len(currentText)-1]), " \t\n") {
				return false
			}

			c.completionType = CompletionTypeSkill
		default:
			return false
		}

		c.userText += keyVal

		return true
	}

	// TODO: handle tab/up(?) to fill in suggested text
	if msg.Type == tea.KeyBackspace {
		if c.userText != "" {
			c.userText = c.userText[:len(c.userText)-1]
		} else {
			c.Reset()
		}
	} else if msg.Type != tea.KeyTab {
		c.userText += keyVal
	}

	// user has cleared the whole completion text
	if c.userText == "" {
		c.completionType = CompletionTypeNone
		c.suggestedText = ""
	}

	return true
}

func (c *CompletionState) CompletionText() string {
	var options []string

	switch c.completionType {
	case CompletionTypeNone:
		return ""
	case CompletionTypeCommand:
		options = c.commandCompleter(strings.TrimPrefix(c.userText, "/"))

	case CompletionTypeSkill:
		options = c.skillCompleter(strings.TrimPrefix(c.userText, "$"))

	default:
		return ""
	}

	if len(options) == 0 {
		c.suggestedText = ""
		return ""
	}

	strippedUserText := strings.TrimLeftFunc(c.userText, func(r rune) bool { return r == '/' || r == '$' })
	c.suggestedText = strings.TrimPrefix(options[0], strippedUserText)

	return c.userText + c.suggestedText
}

func (c *CompletionState) Reset() {
	c.userText = ""
	c.suggestedText = ""
	c.completionType = CompletionTypeNone
}
