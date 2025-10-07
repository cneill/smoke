package input

type ResizeMessage struct{}

type UserMessage struct {
	SourceID string
	Content  string
}

type CancelUserMessage struct {
	SourceID string
	Err      error
}
