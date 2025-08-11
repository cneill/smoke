package claude

import (
	"log/slog"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/cneill/smoke/pkg/llms"
)

func GetModel(search string, defaultModel anthropic.Model) anthropic.Model {
	aliases := llms.ModelAliases[anthropic.Model]{
		anthropic.ModelClaudeOpus4_1_20250805: []string{"opus", "opus4.1"},
		anthropic.ModelClaudeOpus4_0:          []string{"opus4"},
		anthropic.ModelClaudeSonnet4_0:        []string{"sonnet", "sonnet4"},
	}

	if model := aliases.Match(search); model != "" {
		return model
	}

	slog.Warn("model not found, using default", "search", search)

	return defaultModel
}
