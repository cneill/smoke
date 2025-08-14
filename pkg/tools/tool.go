package tools

import "fmt"

type Tool interface {
	Name() string
	Description() string
	Params() Params
	Run(args Args) (string, error)
}

type Tools []Tool

func (t Tools) Params(name string) (Params, error) {
	for _, tool := range t {
		if tool.Name() == name {
			return tool.Params(), nil
		}
	}

	return Params{}, ErrUnknownTool
}

func (t Tools) Call(name string, args Args) (string, error) {
	for _, tool := range t {
		if tool.Name() == name {
			output, err := tool.Run(args)
			if err != nil {
				return "", fmt.Errorf("%w: %w", ErrCallFailed, err)
			}

			return output, nil
		}
	}

	return "", ErrUnknownTool
}
