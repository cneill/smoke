package smoke

import "errors"

var (
	ErrOptions   = errors.New("options error")
	ErrNoSession = errors.New("no main session found")
	ErrMaxTurns  = errors.New("exceeded maximum number of tool-call turns")
)
