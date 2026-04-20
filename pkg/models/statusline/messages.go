package statusline

type Message interface {
	isStatuslineMessage()
}

type SuggestionMessage struct {
	CompletionText string
}

func (s SuggestionMessage) isStatuslineMessage() {}

type UsageMessage struct {
	InputTokens  int64
	OutputTokens int64
}

func (u UsageMessage) isStatuslineMessage() {}
