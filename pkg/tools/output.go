package tools

type OutputType string

const (
	OutputTypeUnknown = "unknown"
	OutputTypeText    = "text"
	OutputTypeImage   = "image"
)

type Output struct {
	Text      string
	ImagePath string
}

func (o *Output) Type() OutputType {
	switch {
	case o.ImagePath != "":
		return OutputTypeImage
	case o.Text != "":
		return OutputTypeText
	default:
		return OutputTypeUnknown
	}
}
