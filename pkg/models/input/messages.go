package input

type ResizeMessage struct{}

type UserMessage struct {
	Content string
}

type ExitCommand struct{}

type UnknownCommand struct {
	Command string
	Args    []string
}
