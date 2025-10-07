package llms

type Event interface {
	isEvent() // expand later?
}

type EventTextDelta struct {
	Text string
}

func (e EventTextDelta) isEvent() {}

type EventToolCallsRequested struct {
	Calls []ToolCall
}

func (e EventToolCallsRequested) isEvent() {}

// TODO: need this?
type EventToolCallResults struct {
	Messages []*Message
}

func (e EventToolCallResults) isEvent() {}

type EventUsageUpdate struct {
	InputTokens  int64
	OutputTokens int64
}

func (e EventUsageUpdate) isEvent() {}

type EventFinalMessage struct {
	Message *Message
}

func (e EventFinalMessage) isEvent() {}

type EventError struct {
	Err error
}

func (e EventError) isEvent() {}

type EventDone struct{}

func (e EventDone) isEvent() {}
