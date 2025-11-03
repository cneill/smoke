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
	if o.Text != "" {
		return OutputTypeText
	} else if o.ImagePath != "" {
		return OutputTypeImage
	}

	return OutputTypeUnknown
}
