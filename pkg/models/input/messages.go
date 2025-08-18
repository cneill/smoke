package input

type ResizeMessage struct{}

type UserMessage struct {
	Content string
}

type CancelUserMessage struct {
	Err error
}
