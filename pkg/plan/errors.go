package plan

import "errors"

var (
	ErrInvalidJSON             = errors.New("invalid JSON")
	ErrUnknownItemType         = errors.New("unknown item type")
	ErrInvalidTaskItem         = errors.New("invalid task item")
	ErrInvalidContextItem      = errors.New("invalid context item")
	ErrInvalidCompletionItem   = errors.New("invalid completion item")
	ErrUnknownOperation        = errors.New("unknown item operation")
	ErrUnknownContextType      = errors.New("unknown context type")
	ErrUnknownCompletionStatus = errors.New("unknown completion status")
)
