package llms

import "context"

type Conversation interface {
	ID() string
	Events() <-chan Event
	Start(ctx context.Context)
	Cancel(err error)
	Continue(ctx context.Context) error
	Close()
}
