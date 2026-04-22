package input

type Message interface {
	isInputMessage()
}

type ResizeMessage struct{}

func (r ResizeMessage) isInputMessage() {}

type UserMessage struct {
	SourceID string
	Content  string
}

func (u UserMessage) isInputMessage() {}

type CancelUserMessage struct {
	SourceID string
	Err      error
}

func (c CancelUserMessage) isInputMessage() {}

type ShiftModeMessage struct{}

func (s ShiftModeMessage) isInputMessage() {}

type CompletionMessage struct {
	Text string
}

func (c CompletionMessage) isInputMessage() {}
