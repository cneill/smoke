package tools

import "errors"

var (
	ErrArguments         = errors.New("arguments error")
	ErrCallFailed        = errors.New("tool call failed")
	ErrUnknownTool       = errors.New("unknown tool")
	ErrFileSystem        = errors.New("file system error")
	ErrMissingExecutable = errors.New("missing executable")

	ErrInvalidJSON      = errors.New("invalid JSON")
	ErrUnknownKeys      = errors.New("unknown argument keys")
	ErrMissingKeys      = errors.New("missing required keys")
	ErrWrongTypeKeys    = errors.New("keys with wrong types")
	ErrCommandExecution = errors.New("error executing command")
	ErrUnexpectedValue  = errors.New("unexpected value")
)
