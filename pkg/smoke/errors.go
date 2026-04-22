package smoke

import "errors"

var (
	ErrOptions   = errors.New("options error")
	ErrNoSession = errors.New("no main session found")
)
