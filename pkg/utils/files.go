package utils

import (
	"bytes"
	"fmt"
	"strconv"
)

// WithLineNumbers takes multiple lines of text and returns a string that prefixes them with their line numbers. If
// 'start' is supplied, the count starts there.
// func WithLineNumbers(contents string, start ...int) string {
func WithLineNumbers(lines [][]byte, start ...int) []byte {
	if len(lines) == 0 {
		return nil
	}

	maxLine := len(lines)
	if len(start) > 0 {
		maxLine = start[0] + len(lines)
	}

	width := len(strconv.Itoa(maxLine))
	buf := &bytes.Buffer{}

	correction := 1
	if len(start) > 0 {
		correction = start[0]
	}

	for i, line := range lines {
		if i == len(lines)-1 && len(line) == 0 {
			break
		}

		fmt.Fprintf(buf, "%*d: %s\n", width, i+correction, line)
	}

	return buf.Bytes()
}
