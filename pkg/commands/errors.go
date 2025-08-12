package commands

import "errors"

var (
	ErrArguments      = errors.New("arguments error")
	ErrRun            = errors.New("run error")
	ErrUnknownCommand = errors.New("unknown command")
)
