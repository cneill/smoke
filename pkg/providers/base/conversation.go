// Package base provides shared conversation scaffolding for LLM provider implementations.
package base

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/cneill/smoke/pkg/llms"
)

const maxIterations = 2048

// ConversationOpts holds the parameters required to construct a Conversation.
type ConversationOpts struct {
	Session      *llms.Session
	LLMInfo      *llms.LLMInfo
	Config       *llms.Config
	Stream       bool
	SendStream   func(ctx context.Context) error
	SendNoStream func(ctx context.Context) error
}

// OK validates that all required fields are present.
func (o *ConversationOpts) OK() error {
	switch {
	case o.Session == nil:
		return fmt.Errorf("missing session")
	case o.LLMInfo == nil:
		return fmt.Errorf("missing LLM info")
	case o.Config == nil:
		return fmt.Errorf("missing config")
	case o.SendStream == nil:
		return fmt.Errorf("missing SendStream func")
	case o.SendNoStream == nil:
		return fmt.Errorf("missing SendNoStream func")
	}

	return nil
}

// Conversation is the shared scaffolding for provider conversation implementations. It manages the
// event channel, continue channel, cancellation, and the main iteration loop. Provider-specific
// structs embed *Conversation and supply their send logic via ConversationOpts.
type Conversation struct {
	session      *llms.Session
	llmInfo      *llms.LLMInfo
	config       *llms.Config
	stream       bool
	cancel       context.CancelCauseFunc
	eventChan    chan llms.Event
	continueChan chan struct{}

	sendStream   func(ctx context.Context) error
	sendNoStream func(ctx context.Context) error

	HasPendingToolCalls bool
}

// NewConversation constructs and initializes a Conversation from opts, derives a child context with
// cancellation, and returns both. The caller is responsible for launching Start(newCtx) as a
// goroutine.
func NewConversation(ctx context.Context, opts *ConversationOpts) (*Conversation, context.Context, error) {
	if err := opts.OK(); err != nil {
		return nil, nil, fmt.Errorf("conversation opts error: %w", err)
	}

	newCtx, cancel := context.WithCancelCause(ctx)

	conv := &Conversation{
		session:      opts.Session,
		llmInfo:      opts.LLMInfo,
		config:       opts.Config,
		stream:       opts.Stream,
		cancel:       cancel,
		eventChan:    make(chan llms.Event),
		continueChan: make(chan struct{}),
		sendStream:   opts.SendStream,
		sendNoStream: opts.SendNoStream,
	}

	return conv, newCtx, nil
}

// Session returns the session associated with this conversation.
func (c *Conversation) Session() *llms.Session { return c.session }

// Config returns the LLM config associated with this conversation.
func (c *Conversation) Config() *llms.Config { return c.config }

// LLMInfo returns the LLM info associated with this conversation.
func (c *Conversation) LLMInfo() *llms.LLMInfo { return c.llmInfo }

// ID satisfies [llms.Conversation].
func (c *Conversation) ID() string { return c.session.Name }

// Events satisfies [llms.Conversation].
func (c *Conversation) Events() <-chan llms.Event { return c.eventChan }

// Cancel satisfies [llms.Conversation].
func (c *Conversation) Cancel(err error) { c.cancel(err) }

// Continue satisfies [llms.Conversation].
func (c *Conversation) Continue(ctx context.Context) error {
	select {
	case c.continueChan <- struct{}{}:
		return nil
	case <-ctx.Done():
		return fmt.Errorf("conversation context error: %w", ctx.Err())
	}
}

// Close satisfies [llms.Conversation].
func (c *Conversation) Close() { c.cancel(nil) }

// Emit sends an event to the event channel, or returns early if the context is cancelled.
func (c *Conversation) Emit(ctx context.Context, e llms.Event) {
	select {
	case c.eventChan <- e:
	case <-ctx.Done():
	}
}

// WaitForContinue blocks until Continue is called or the context is cancelled.
func (c *Conversation) WaitForContinue(ctx context.Context) error {
	select {
	case <-c.continueChan:
		return nil
	case <-ctx.Done():
		return fmt.Errorf("context error while waiting for continue: %w", ctx.Err())
	}
}

// NewMessage constructs a new [llms.Message] pre-stamped with this conversation's LLMInfo.
func (c *Conversation) NewMessage(opts ...llms.MessageOpt) *llms.Message {
	msg := llms.NewMessage(
		llms.WithLLMInfo(c.llmInfo),
	)

	for _, opt := range opts {
		msg = opt(msg)
	}

	return msg
}

// Start runs the main conversation loop. It dispatches to sendStream or sendNoStream on each
// iteration, handles tool call continuation, and emits EventDone when finished. It should be
// launched as a goroutine by the provider's StartConversation method.
func (c *Conversation) Start(ctx context.Context) {
	defer close(c.eventChan)

	for range maxIterations {
		var err error
		if c.stream {
			err = c.sendStream(ctx)
		} else {
			err = c.sendNoStream(ctx)
		}

		if err != nil {
			mode := "non-streaming"
			if c.stream {
				mode = "streaming"
			}

			c.Emit(ctx, llms.EventError{
				Err: fmt.Errorf("failed to send message (%s): %w", mode, err),
			})
		}

		if !c.HasPendingToolCalls {
			break
		}

		if err := c.WaitForContinue(ctx); err != nil {
			c.Emit(ctx, llms.EventError{
				Err: fmt.Errorf("failed while waiting for tool call results: %w", err),
			})
		}

		// TODO: this COULD return unrelated runs if Smoke messes up - need to check this?
		callMessages := c.session.LastRunByRole(llms.RoleTool)

		c.Emit(ctx, llms.EventToolCallResults{
			Messages: callMessages,
		})
	}

	select {
	case c.eventChan <- llms.EventDone{}:
	case <-ctx.Done():
		slog.Debug("context cancelled before sending done event", "error", ctx.Err())
	}
}
