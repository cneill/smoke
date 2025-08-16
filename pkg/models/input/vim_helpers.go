package input

import "unicode"

// Helper functions for word motions
func isWordChar(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_'
}

func isWhitespace(r rune) bool {
	return unicode.IsSpace(r)
}

// findNextWord finds the start of the next word (lowercase w motion)
func findNextWord(content string, pos int) int {
	runes := []rune(content)

	length := len(runes)
	if pos >= length {
		return pos
	}

	// Skip current word/punctuation
	inWord := pos < length && isWordChar(runes[pos])
	for pos < length && ((inWord && isWordChar(runes[pos])) || (!inWord && !isWhitespace(runes[pos]) && !isWordChar(runes[pos]))) {
		pos++
	}

	// Skip whitespace
	for pos < length && isWhitespace(runes[pos]) {
		pos++
	}

	return pos
}

// findNextWORD finds the start of the next WORD (uppercase W motion)
func findNextWORD(content string, pos int) int {
	runes := []rune(content)

	length := len(runes)
	if pos >= length {
		return pos
	}

	// Skip current WORD (non-whitespace)
	for pos < length && !isWhitespace(runes[pos]) {
		pos++
	}

	// Skip whitespace
	for pos < length && isWhitespace(runes[pos]) {
		pos++
	}

	return pos
}

// findEndOfWord finds the end of current/next word (lowercase e motion)
func findEndOfWord(content string, pos int) int {
	runes := []rune(content)

	length := len(runes)
	if pos >= length-1 {
		return length - 1
	}

	pos++ // Move at least one character

	// Skip whitespace
	for pos < length && isWhitespace(runes[pos]) {
		pos++
	}

	if pos >= length {
		return length - 1
	}

	// Move to end of word/punctuation
	inWord := isWordChar(runes[pos])
	for pos < length-1 && ((inWord && isWordChar(runes[pos+1])) || (!inWord && !isWhitespace(runes[pos+1]) && !isWordChar(runes[pos+1]))) {
		pos++
	}

	return pos
}

// findEndOfWORD finds the end of current/next WORD (uppercase E motion)
func findEndOfWORD(content string, pos int) int {
	runes := []rune(content)

	length := len(runes)
	if pos >= length-1 {
		return length - 1
	}

	pos++ // Move at least one character

	// Skip whitespace
	for pos < length && isWhitespace(runes[pos]) {
		pos++
	}

	// Move to end of WORD
	for pos < length-1 && !isWhitespace(runes[pos+1]) {
		pos++
	}

	return pos
}

// findPrevWord finds the start of the previous word (lowercase b motion)
func findPrevWord(content string, pos int) int {
	runes := []rune(content)

	if pos <= 0 {
		return 0
	}

	pos-- // Move at least one character back

	// Skip whitespace
	for pos > 0 && isWhitespace(runes[pos]) {
		pos--
	}

	if pos <= 0 {
		return 0
	}

	// Move to start of word/punctuation
	inWord := isWordChar(runes[pos])
	for pos > 0 && ((inWord && isWordChar(runes[pos-1])) || (!inWord && !isWhitespace(runes[pos-1]) && !isWordChar(runes[pos-1]))) {
		pos--
	}

	return pos
}

// findPrevWORD finds the start of the previous WORD (uppercase B motion)
func findPrevWORD(content string, pos int) int {
	runes := []rune(content)

	if pos <= 0 {
		return 0
	}

	pos-- // Move at least one character back

	// Skip whitespace
	for pos > 0 && isWhitespace(runes[pos]) {
		pos--
	}

	// Move to start of WORD
	for pos > 0 && !isWhitespace(runes[pos-1]) {
		pos--
	}

	return pos
}
