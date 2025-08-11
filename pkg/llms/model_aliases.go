package llms

type ModelAliases[T ~string] map[T][]string

func (m ModelAliases[T]) Match(search string) T {
	temp := T(search)

	for model, aliases := range m {
		if model == temp {
			return model
		}

		for _, alias := range aliases {
			if typed := T(alias); typed == temp {
				return model
			}
		}
	}

	return T("")
}
