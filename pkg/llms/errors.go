package llms

import "errors"

var (
	ErrCompletion    = errors.New("completion error")
	ErrEmptyResponse = errors.New("empty response")
	ErrPromptRefused = errors.New("prompt refused")
	ErrNoToolCalls   = errors.New("no tool calls in message")
)
