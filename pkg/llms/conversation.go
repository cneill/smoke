package llms

import "context"

type Conversation interface {
	ID() string
	Events() <-chan Event
	Cancel(err error)
	Continue(ctx context.Context) error
	Close()
}
