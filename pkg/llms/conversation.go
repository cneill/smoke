package llms

import "context"

type Conversation interface {
	ID() string
	Events() <-chan Event
	Cancel()
	Continue(ctx context.Context) error
	Close() error
}
