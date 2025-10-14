package llms

import "context"

type Conversation interface {
	ID() string
	Events() <-chan Event
	Cancel(err error) // TODO: merge into Close?
	Continue(ctx context.Context) error
	Close()
}
