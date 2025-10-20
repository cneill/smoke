package utils

import "math/rand/v2"

const idChars = "abcdef0123456789"

// RandID returns a random 32-character hex string
func RandID(size int) string {
	output := []byte{}
	for range size {
		output = append(output, idChars[rand.IntN(len(idChars))]) //nolint:gosec
	}

	return string(output)
}
