package llms

import "errors"

var (
	ErrCompletion    = errors.New("completion error")
	ErrEmptyResponse = errors.New("empty response")
	ErrPromptRefused = errors.New("prompt refused")
	ErrNoToolCalls   = errors.New("no tool calls in message")
	ErrMaxTokens     = errors.New("response exceeded max tokens")
	ErrMaxContext    = errors.New("request or response exceeded context window length")
)
