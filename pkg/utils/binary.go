package utils

import (
	"bytes"
	"unicode/utf8"
)

// IsBinary checks if the given byte slice contains binary data by looking for null bytes.
func IsBinary(data []byte) bool {
	checkSize := min(len(data), 8192)
	sample := data[:checkSize]

	// This might be too restrictive, but probably not.
	if bytes.IndexByte(sample, 0) != -1 {
		return true
	}

	return !utf8.Valid(sample)
}
