package llms

import (
	"fmt"
	"sync"
	"time"

	"github.com/cneill/smoke/pkg/tools"
)

type Session struct {
	Name          string    `json:"name"`
	CreatedAt     time.Time `json:"created_at"`
	SystemMessage string    `json:"system_message"`
	// Should we add the system message to the Messages slice?
	SystemAsMessage bool         `json:"system_as_message"`
	Messages        []*Message   `json:"messages"`
	messageMutex    sync.RWMutex `json:"-"`

	Tools *tools.Manager `json:"-"`

	usageMutex   sync.RWMutex `json:"-"`
	InputTokens  int64        `json:"input_tokens"`
	OutputTokens int64        `json:"output_tokens"`
}

type SessionOpts struct {
	Name            string
	SystemMessage   string
	SystemAsMessage bool
	Tools           *tools.Manager
}

func (s *SessionOpts) OK() error {
	switch {
	case s.Name == "":
		return fmt.Errorf("missing session name")
	case s.SystemMessage == "":
		return fmt.Errorf("missing system message")
	case s.Tools == nil:
		return fmt.Errorf("missing tools manager")
	}

	return nil
}

// NewSession returns an initialized Session ready for messages.
func NewSession(opts *SessionOpts) (*Session, error) {
	if err := opts.OK(); err != nil {
		return nil, fmt.Errorf("session options error: %w", err)
	}

	session := &Session{
		Name:            opts.Name,
		SystemMessage:   opts.SystemMessage,
		SystemAsMessage: opts.SystemAsMessage,
		Messages:        []*Message{},
		Tools:           opts.Tools,

		CreatedAt: time.Now(),

		messageMutex: sync.RWMutex{},
		usageMutex:   sync.RWMutex{},
	}

	if err := session.SetSystemMessage(opts.SystemMessage); err != nil {
		return nil, fmt.Errorf("failed to set system message: %w", err)
	}

	return session, nil
}

// SetSystemMessage sets the Session's system message, modifying the existing Message if necessary.
func (s *Session) SetSystemMessage(system string) error {
	s.SystemMessage = system

	// If we have an existing system message in the message log, replace it
	existingSystemMessage := false

	s.messageMutex.Lock()

	for messageIdx, message := range s.Messages {
		if message.Role == RoleSystem {
			existingSystemMessage = true
			newMessage := message.Clone()
			newMessage.Content = system
			newMessage.Added = time.Now()
			s.Messages[messageIdx] = newMessage

			break
		}
	}

	s.messageMutex.Unlock()

	// If we don't already have one, and if we need to provide one for this LLM provider, add it
	if s.SystemAsMessage && !existingSystemMessage {
		systemMsg := SimpleMessage(RoleSystem, s.SystemMessage)
		if err := s.AddMessage(systemMsg); err != nil {
			return fmt.Errorf("failed to add system message: %w", err)
		}
	}

	return nil
}

// AddMessage adds an existing Message to the Session as-is.
func (s *Session) AddMessage(msg *Message) error {
	if err := msg.OK(); err != nil {
		return fmt.Errorf("failed to add message to session: %w", err)
	}

	s.messageMutex.Lock()
	defer s.messageMutex.Unlock()

	s.Messages = append(s.Messages, msg)

	return nil
}

// LastByRole returns the most recent message from the specified Role, or nil if there are none.
func (s *Session) LastByRole(role Role) *Message {
	s.messageMutex.RLock()
	defer s.messageMutex.RUnlock()

	for i := len(s.Messages) - 1; i > 0; i-- {
		if s.Messages[i].Role == role {
			return s.Messages[i]
		}
	}

	return nil
}

func (s *Session) Last() *Message {
	if len(s.Messages) == 0 {
		return nil
	}

	s.messageMutex.RLock()
	defer s.messageMutex.RUnlock()

	return s.Messages[len(s.Messages)-1]
}

// UpdateUsage should be called with the total number of input tokens and the number of output tokens from the latest
// response.
func (s *Session) UpdateUsage(inputTokens, outputTokens int64) {
	s.usageMutex.Lock()
	defer s.usageMutex.Unlock()

	s.InputTokens += inputTokens
	s.OutputTokens += outputTokens
}

func (s *Session) Usage() (inputTokens, outputTokens int64) { //nolint:nonamedreturns
	s.usageMutex.RLock()
	defer s.usageMutex.RUnlock()

	return s.InputTokens, s.OutputTokens
}

type MessageCount struct {
	UserMessages      int
	AssistantMessages int
	ToolCallMessages  int
}

func (s *Session) MessageCount() MessageCount {
	s.messageMutex.RLock()
	defer s.messageMutex.RUnlock()

	result := MessageCount{}

	for _, msg := range s.Messages {
		switch msg.Role { //nolint:exhaustive
		case RoleAssistant:
			result.AssistantMessages++
		case RoleTool:
			result.ToolCallMessages++
		case RoleUser:
			result.UserMessages++
		}
	}

	return result
}

// ReplaceMessages searches the Session's messages for messages with the same IDs as those in 'searches' and keeps up
// with the last index, adding the messages from 'replacements' after that index. This assumes that the messages in
// 'searches' are in the same order as they would appear in the Session.
func (s *Session) ReplaceMessages(searches, replacements []*Message) {
	s.messageMutex.Lock()
	defer s.messageMutex.Unlock()

	newMessages := make([]*Message, 0, len(s.Messages))
	lastIndex := len(s.Messages)

	for _, msg := range s.Messages {
		found := false

		for _, search := range searches {
			if msg.ID == search.ID {
				found = true
				lastIndex = len(newMessages) - 1

				break
			}
		}

		if !found {
			newMessages = append(newMessages, msg)
		}
	}

	if lastIndex >= len(s.Messages)-1 {
		newMessages = append(newMessages, replacements...)
	} else {
		tempMessages := append([]*Message{}, newMessages[:lastIndex+1]...)
		tempMessages = append(tempMessages, replacements...)
		tempMessages = append(tempMessages, newMessages[lastIndex+1:]...)
		newMessages = tempMessages
	}

	s.Messages = newMessages
}

func (s *Session) Teardown() error {
	if s.Tools != nil {
		if err := s.Tools.Teardown(); err != nil {
			return fmt.Errorf("session %q teardown failed: %w", s.Name, err)
		}
	}

	return nil
}
