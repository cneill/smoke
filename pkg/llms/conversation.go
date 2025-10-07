package llms

import "context"

type Conversation interface {
	ID() string
	Events() <-chan Event
	Continue(ctx context.Context) error
	Close() error
}
