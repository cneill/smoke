package main

import (
	"github.com/cneill/smoke/pkg/llms"
)

// apiKeyInfo holds the CLI flag/env var names used to supply provider credentials. This is kept
// separate from pkg/providers, which only knows about provider/model/effort metadata and has no
// concept of CLI flags.
type apiKeyInfo struct {
	apiKeyFlag   string
	apiKeyEnvVar string
	baseURLFlag  string
}

// apiKeyInfos maps a provider name (see llms.LLMType constants) to its apiKeyInfo.
var apiKeyInfos = map[string]apiKeyInfo{ //nolint:gochecknoglobals
	llms.LLMTypeChatGPT: {
		apiKeyFlag:   FlagOpenAIKey,
		apiKeyEnvVar: EnvOpenAIKey,
	},
	llms.LLMTypeClaude: {
		apiKeyFlag:   FlagAnthropicKey,
		apiKeyEnvVar: EnvAnthropicKey,
	},
	llms.LLMTypeGrok: {
		apiKeyFlag:   FlagXAIKey,
		apiKeyEnvVar: EnvXAIKey,
	},
	llms.LLMTypeOllama: {
		// No API key required; model must be specified explicitly via --model.
		baseURLFlag: FlagOllamaHost,
	},
}
