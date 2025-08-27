package grok

import (
	"log/slog"

	"github.com/cneill/smoke/pkg/llms"
)

// TODO: create a custom Model type?

func GetModel(search string, defaultModel string) string {
	aliases := llms.ModelAliases[string]{
		"grok-2-1212":      []string{"2", "grok2", "grok-2"},
		"grok-3":           []string{"3", "grok3", "grok-3"},
		"grok-3-fast":      []string{"3-fast", "grok3-fast", "grok-3-fast"},
		"grok-3-mini":      []string{"3-mini", "grok3-mini", "grok-3-mini"},
		"grok-4-0709":      []string{"4", "grok4", "grok-4"},
		"grok-code-fast-1": []string{"code", "code-fast", "grok-code"},
	}

	if model := aliases.Match(search); model != "" {
		return model
	}

	slog.Warn("model not found, using default", "search", search)

	return defaultModel
}
