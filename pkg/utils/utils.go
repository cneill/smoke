// Package utils contains miscellaneous helper functions used by different parts of the application.
package utils

import "math/rand/v2"

const idChars = "abcdef0123456789"

// RandID returns a random 16-character hex string
func RandID() string {
	output := []byte{}
	for range 16 {
		output = append(output, idChars[rand.IntN(len(idChars))]) //nolint:gosec
	}

	return string(output)
}
