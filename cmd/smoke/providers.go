package main

import (
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/cneill/smoke/pkg/llms"
	"github.com/openai/openai-go/v3"
)

var providers = providerMappings{ //nolint:gochecknoglobals
	llms.LLMTypeChatGPT: {
		apiKeyFlag:   FlagOpenAIKey,
		apiKeyEnvVar: EnvOpenAIKey,
		defaultModel: openai.ChatModelGPT5_4,
		aliases: modelAliases{
			openai.ChatModelGPT4:        {"4", "gpt4", "gpt-4"},
			openai.ChatModelGPT4o:       {"4o", "gpt4o", "gpt-4o"},
			openai.ChatModelGPT4oMini:   {"4o-mini", "gpt4o-mini", "gpt-4o-mini"},
			openai.ChatModelGPT4_1:      {"4.1", "gpt4.1", "gpt-4.1"},
			openai.ChatModelGPT4_1Mini:  {"4.1-mini", "gpt4.1-mini", "gpt-4.1-mini"},
			openai.ChatModelGPT4_1Nano:  {"4.1-nano", "gpt4.1-nano", "gpt-4.1-nano"},
			openai.ChatModelGPT5:        {"5", "gpt5", "gpt-5"},
			openai.ChatModelGPT5Mini:    {"5-mini", "gpt5-mini", "gpt-5-mini"},
			openai.ChatModelGPT5Nano:    {"5-nano", "gpt5-nano", "gpt-5-nano"},
			openai.ChatModelGPT5_1:      {"5.1", "gpt5.1", "gpt-5.1"},
			openai.ChatModelGPT5_1Mini:  {"5.1-mini", "gpt5.1-mini", "gpt-5.1-mini"},
			openai.ChatModelGPT5_1Codex: {"5.1-codex", "gpt5.1-codex", "gpt-5.1-codex"},
			openai.ChatModelGPT5_2:      {"5.2", "gpt5.2", "gpt-5.2"},
			openai.ChatModelGPT5_2Pro:   {"5.2-pro", "gpt5.2-pro", "gpt-5.2-pro"},
			openai.ChatModelGPT5_4:      {"5.4", "gpt5.4", "gpt-5.4"},
			openai.ChatModelGPT5_4Mini:  {"5.4-mini", "gpt5.4-mini", "gpt-5.4-mini"},
			openai.ChatModelGPT5_4Nano:  {"5.4-nano", "gpt5.4-nano", "gpt-5.4-nano"},
			openai.ChatModelO1:          {"o1", "gpto1", "gpt-o1"},
			openai.ChatModelO1Mini:      {"o1-mini", "gpto1-mini", "gpt-o1-mini"},
			openai.ChatModelO1Preview:   {"o1-preview", "gpto1-preview", "gpt-o1-preview"},
			openai.ChatModelO3:          {"o3", "gpto3", "gpt-o3"},
			openai.ChatModelO3Mini:      {"o3-mini", "gpto3-mini", "gpt-o3-mini"},
			openai.ChatModelO4Mini:      {"o4-mini", "gpto4-mini", "gpt-o4-mini"},
		},
	},
	llms.LLMTypeClaude: {
		apiKeyFlag:   FlagAnthropicKey,
		apiKeyEnvVar: EnvAnthropicKey,
		defaultModel: anthropic.ModelClaudeSonnet4_6,
		aliases: modelAliases{
			anthropic.ModelClaudeOpus4_7:          {"opus", "opus4.7", "o47"},
			anthropic.ModelClaudeOpus4_6:          {"opus4.6", "o46"},
			anthropic.ModelClaudeOpus4_5:          {"opus4.5", "o45"},
			anthropic.ModelClaudeOpus4_1_20250805: {"opus4.1", "o41"},
			anthropic.ModelClaudeSonnet4_6:        {"sonnet", "sonnet4.6", "s46"},
			anthropic.ModelClaudeSonnet4_5:        {"sonnet4.5", "s45"},
			anthropic.ModelClaudeHaiku4_5:         {"haiku", "haiku4.5", "h45"},
		},
	},
	llms.LLMTypeGrok: {
		apiKeyFlag:   FlagXAIKey,
		apiKeyEnvVar: EnvXAIKey,
		defaultModel: "grok-4.20-0309-reasoning",
		aliases: modelAliases{
			"grok-4.20-0309-reasoning": {"4.2", "420"},
			"grok-4-1-fast-reasoning":  {"4.1", "fast"},
		},
	},
	llms.LLMTypeOllama: {
		// No API key required; model must be specified explicitly via --model.
		apiKeyFlag:   "",
		apiKeyEnvVar: "",
		baseURLFlag:  FlagOllamaHost,
		defaultModel: "",
		aliases:      modelAliases{},
	},
}

// TODO: make this customizable by the user in config?
func getProviders() providerMappings {
	return providers
}

type providerMappings map[string]*providerDetails

func (p providerMappings) names() []string {
	names := slices.Collect(maps.Keys(p))
	slices.Sort(names)

	return names
}

func (p providerMappings) details(provider string) (*providerDetails, error) {
	details, ok := p[provider]
	if !ok {
		return nil, fmt.Errorf("unknown model provider %q, must choose one of %s", provider, strings.Join(p.names(), ", "))
	}

	return details, nil
}

type providerDetails struct {
	apiKeyFlag   string
	apiKeyEnvVar string
	baseURLFlag  string
	defaultModel string
	aliases      modelAliases
}

func (p providerDetails) getModel(search string) (string, error) {
	// Provider has no predefined models (e.g. ollama): pass through verbatim.
	if len(p.aliases) == 0 {
		if search == "" {
			return "", fmt.Errorf("this provider requires --model to be specified explicitly")
		}

		return search, nil
	}

	for model, aliases := range p.aliases {
		if model == search {
			return model, nil
		}

		if slices.Contains(aliases, search) {
			return model, nil
		}
	}

	if search == "" {
		return p.defaultModel, nil
	}

	return "", fmt.Errorf("unknown model: %q\n\n%s", search, p.aliases)
}

type modelAliases map[string][]string

func (m modelAliases) String() string {
	builder := strings.Builder{}
	builder.Grow(64)
	builder.WriteString("Model aliases:\n")

	modelNames := slices.Collect(maps.Keys(m))
	slices.Sort(modelNames)

	for _, modelName := range modelNames {
		builder.WriteString(modelName)
		builder.WriteString(": ")
		builder.WriteString(strings.Join(m[modelName], ", "))
		builder.WriteByte('\n')
	}

	return builder.String()
}
