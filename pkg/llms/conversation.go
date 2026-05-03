package llms

import "context"

type Conversation interface {
	ID() string
	Events() <-chan Event
	Start(ctx context.Context)
	Cancel(err error) // TODO: merge into Close?
	Continue(ctx context.Context) error
	Close()
}
