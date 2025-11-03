package utils

// ToStrings takes a slice of string-like types and returns a slice of strings.
func ToStrings[T ~string](input []T) []string {
	result := make([]string, len(input))
	for i, stringLike := range input {
		result[i] = string(stringLike)
	}

	return result
}
