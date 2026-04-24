package elicit

type Message interface {
	isElicitMessage()
}

type RequestMessage struct {
	Request Request
}

func (r RequestMessage) isElicitMessage() {}
