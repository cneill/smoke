package llms

import (
	"fmt"
	"sync"
	"time"
)

type Session struct {
	Name          string     `json:"name"`
	SystemMessage string     `json:"system_message"`
	Messages      []*Message `json:"messages"`

	messageMutex sync.RWMutex
}

type SessionOpts struct {
	Name          string
	SystemMessage string
}

func (s *SessionOpts) OK() error {
	if s.Name == "" {
		return fmt.Errorf("must give sessions names")
	}

	return nil
}

// NewSession returns an initialized Session ready for messages.
func NewSession(opts *SessionOpts) (*Session, error) {
	if err := opts.OK(); err != nil {
		return nil, fmt.Errorf("session options error: %w", err)
	}

	session := &Session{
		Name:          opts.Name,
		SystemMessage: opts.SystemMessage,
		Messages:      []*Message{},

		messageMutex: sync.RWMutex{},
	}

	return session, nil
}

// SetSystemMessage sets the Session's system message, modifying the existing Message if necessary.
func (s *Session) SetSystemMessage(system string) {
	s.SystemMessage = system

	s.messageMutex.Lock()
	defer s.messageMutex.Unlock()

	for i, message := range s.Messages {
		if message.Role == RoleSystem {
			s.Messages[i] = &Message{
				Role:    message.Role,
				Content: system,
				Added:   time.Now(),
			}

			break
		}
	}
}

// AddMessage adds an existing Message to the Session as-is.
func (s *Session) AddMessage(msg *Message) {
	s.messageMutex.Lock()
	defer s.messageMutex.Unlock()

	s.Messages = append(s.Messages, msg)
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
