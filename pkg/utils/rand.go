package utils

import "math/rand/v2"

const idChars = "abcdef0123456789"

// RandID returns a random hex string with the number of characters specified in 'size'.
func RandID(size int) string {
	output := make([]byte, size)
	for i := range size {
		output[i] = idChars[rand.IntN(len(idChars))] //nolint:gosec
	}

	return string(output)
}
