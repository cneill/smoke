package utils

import (
	"fmt"
	"strconv"
	"strings"
)

// WithLineNumbers takes multiple lines of text and returns a string that prefixes them with their line numbers. If
// 'start' is supplied, the count starts there.
func WithLineNumbers(contents string, start ...int) string {
	if contents == "" {
		return ""
	}

	lines := strings.Split(contents, "\n")

	maxLine := len(lines)
	if len(start) > 0 {
		maxLine = start[0] + len(lines)
	}

	width := len(strconv.Itoa(maxLine))
	builder := &strings.Builder{}

	correction := 1
	if len(start) > 0 {
		correction = start[0]
	}

	for i, line := range lines {
		if i == len(lines)-1 && line == "" {
			break
		}

		fmt.Fprintf(builder, "%*d: %s\n", width, i+correction, line)
	}

	return builder.String()
}
