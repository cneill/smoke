package statusline

type Message interface {
	isStatuslineMessage()
}
