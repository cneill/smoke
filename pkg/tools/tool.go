package tools

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
			return tool.Run(args)
		}
	}

	return "", ErrUnknownTool
}
