package llms

import "errors"

var (
	ErrCompletion    = errors.New("completion error")
	ErrEmptyResponse = errors.New("empty response")
	ErrPromptRefused = errors.New("prompt refused")
)

type SessionError struct {
	message    *Message
	underlying error
}

func NewSessionError(message *Message, err error) SessionError {
	return SessionError{
		message:    message,
		underlying: err,
	}
}

func (s SessionError) Error() string {
	if s.underlying != nil {
		return s.underlying.Error()
	}

	return ""
}
