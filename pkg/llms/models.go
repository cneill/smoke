package llms

import (
	"slices"
	"strings"
)

// ModelInfo describes a single model: its provider, canonical name, known aliases, and context window size.
type ModelInfo struct {
	Provider            LLMType
	Model               string
	Aliases             []string
	ContextWindowTokens int64
}

// ModelInfos is an ordered list of ModelInfo. Order matters when resolving aliases claimed by more than
// one model: the first match wins.
type ModelInfos []ModelInfo

// String lists each model and its aliases, sorted by model name.
func (m ModelInfos) String() string {
	sorted := slices.SortedFunc(slices.Values(m), func(a, b ModelInfo) int {
		return strings.Compare(a.Model, b.Model)
	})

	sb := strings.Builder{}
	sb.Grow(64)
	sb.WriteString("Model aliases:\n")

	for _, info := range sorted {
		sb.WriteString(info.Model)
		sb.WriteString(": ")
		sb.WriteString(strings.Join(info.Aliases, ", "))
		sb.WriteByte('\n')
	}

	return sb.String()
}
