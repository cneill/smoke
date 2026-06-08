package utils

import (
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

// ToStrings takes a slice of string-like types and returns a slice of strings.
func ToStrings[T ~string](input []T) []string {
	result := make([]string, len(input))
	for i, stringLike := range input {
		result[i] = string(stringLike)
	}

	return result
}

// TitleCase is somehow "better" than the deprecated [strings.Title]. Also handles ~strings.
func TitleCase[T ~string](input T) string {
	return cases.Title(language.English).String(string(input))
}
