package tools

import (
	"encoding/json"
	"fmt"
	"strings"
)

type Example struct {
	Description string
	Args        Args
}

type Examples []Example

func ExampleJSONArguments(args Args) (string, error) {
	exampleJSON, err := json.Marshal(args)
	if err != nil {
		return "", fmt.Errorf("%w: error generating example JSON: %w", ErrInvalidJSON, err)
	}

	return string(exampleJSON), nil
}

func CollectExamples(examples ...Example) string {
	builder := strings.Builder{}
	builder.WriteString("\n## Examples\n\n")

	for _, example := range examples {
		if example.Description != "" {
			builder.WriteString("**Description:** " + example.Description + "\n")
		}

		jsonArgs, err := ExampleJSONArguments(example.Args)
		if err != nil {
			panic(err)
		}

		builder.WriteString("**Arguments:** " + jsonArgs + "\n\n")
	}

	return strings.TrimSuffix(builder.String(), "\n")
}
