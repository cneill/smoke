package main

import (
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/cneill/smoke/pkg/llms"
	"github.com/openai/openai-go/v2"
)

var providers = providerMappings{ //nolint:gochecknoglobals
	llms.LLMTypeChatGPT: {
		apiKeyFlag:   FlagOpenAIKey,
		apiKeyEnvVar: EnvOpenAIKey,
		defaultModel: openai.ChatModelGPT5,
		aliases: modelAliases{
			openai.ChatModelGPT4o:    {"4", "4o", "gpt4o", "gpt-4o"},
			openai.ChatModelGPT4_1:   {"4.1", "gpt4.1", "gpt-4.1"},
			openai.ChatModelGPT5:     {"5", "gpt5", "gpt-5"},
			openai.ChatModelGPT5Mini: {"5-mini", "gpt5-mini", "gpt-5-mini"},
			openai.ChatModelO3:       {"o3", "gpto3", "gpt-o3"},
			openai.ChatModelO3Mini:   {"o3-mini", "gpto3-mini", "gpt-o3-mini"},
		},
	},
	llms.LLMTypeClaude: {
		apiKeyFlag:   FlagAnthropicKey,
		apiKeyEnvVar: EnvAnthropicKey,
		defaultModel: string(anthropic.ModelClaudeSonnet4_0),
		aliases: modelAliases{
			string(anthropic.ModelClaudeOpus4_1_20250805): {"opus", "opus4.1"},
			string(anthropic.ModelClaudeOpus4_0):          {"opus4"},
			string(anthropic.ModelClaudeSonnet4_0):        {"sonnet", "sonnet4"},
		},
	},
	llms.LLMTypeGrok: {
		apiKeyFlag:   FlagXAIKey,
		apiKeyEnvVar: EnvXAIKey,
		defaultModel: "grok-3",
		aliases: modelAliases{
			"grok-2-1212":      {"2", "grok2", "grok-2"},
			"grok-3":           {"3", "grok3", "grok-3"},
			"grok-3-fast":      {"3-fast", "grok3-fast", "grok-3-fast"},
			"grok-3-mini":      {"3-mini", "grok3-mini", "grok-3-mini"},
			"grok-4-0709":      {"4", "grok4", "grok-4"},
			"grok-code-fast-1": {"code", "code-fast", "grok-code"},
		},
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
	defaultModel string
	aliases      modelAliases
}

func (p providerDetails) getModel(search string) (string, error) {
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

	return "", fmt.Errorf("unknown model: %q\n%s", search, p.aliases)
}

type modelAliases map[string][]string

func (m modelAliases) String() string {
	result := "Model aliases:\n"
	for modelName, aliases := range m {
		result += fmt.Sprintf("%s: %s\n", modelName, strings.Join(aliases, ", "))
	}

	return result
}
