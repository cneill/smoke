package statusline

type Message interface {
	isStatuslineMessage()
}

type CompletionMessage struct {
	Text string
}

func (c CompletionMessage) isStatuslineMessage() {}
