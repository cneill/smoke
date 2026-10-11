// Package markdown provides the shared glamour configuration used to render Markdown in the terminal.
package markdown

import (
	"fmt"

	"github.com/charmbracelet/glamour"
)

// NewTermRenderer returns a glamour renderer that word-wraps output to the given width.
func NewTermRenderer(width int) (*glamour.TermRenderer, error) {
	renderer, err := glamour.NewTermRenderer(
		glamour.WithWordWrap(width),
		glamour.WithEmoji(),
		glamour.WithAutoStyle(),
		glamour.WithPreservedNewLines(),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to set up markdown renderer: %w", err)
	}

	return renderer, nil
}
