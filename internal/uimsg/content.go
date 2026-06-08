package uimsg

// HistoryField is a single labeled value displayed in a structured result.
type HistoryField struct {
	Label string
	Value string
}

func NewField(label, value string) HistoryField {
	return HistoryField{
		Label: label,
		Value: value,
	}
}

// HistoryBlockType determines how a structured result block should be rendered.
type HistoryBlockType string

const (
	HistoryBlockFields   HistoryBlockType = "fields"
	HistoryBlockText     HistoryBlockType = "text"
	HistoryBlockMarkdown HistoryBlockType = "markdown"
)

// HistoryBlock is a single ordered block in a structured UI message.
type HistoryBlock struct {
	Type   HistoryBlockType
	Title  string
	Fields []HistoryField
	Text   string
}

// HistoryContent is structured content for a UI message bubble.
type HistoryContent struct {
	Blocks []HistoryBlock
}
