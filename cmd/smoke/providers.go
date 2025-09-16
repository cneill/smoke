package main

import (
	"slices"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/cneill/smoke/pkg/llms"
	"github.com/openai/openai-go/v2"
)

type providerDetails struct {
	flag            string
	envVar          string
	defaultModel    string
	fuzzyModelNames map[string][]string
}

func (p providerDetails) getModel(search string) string {
	for model, aliases := range p.fuzzyModelNames {
		if model == search {
			return model
		}

		if slices.Contains(aliases, search) {
			return model
		}
	}

	return p.defaultModel
}

func providerDetailMappings(provider llms.LLMType) *providerDetails {
	mappings := map[llms.LLMType]*providerDetails{
		llms.LLMTypeChatGPT: {
			flag:         FlagOpenAIKey,
			envVar:       EnvOpenAIKey,
			defaultModel: openai.ChatModelGPT5,
			fuzzyModelNames: map[string][]string{
				openai.ChatModelGPT4o:    {"4", "4o", "gpt4o", "gpt-4o"},
				openai.ChatModelGPT4_1:   {"4.1", "gpt4.1", "gpt-4.1"},
				openai.ChatModelGPT5:     {"5", "gpt5", "gpt-5"},
				openai.ChatModelGPT5Mini: {"5-mini", "gpt5-mini", "gpt-5-mini"},
				openai.ChatModelO3:       {"o3", "gpto3", "gpt-o3"},
				openai.ChatModelO3Mini:   {"o3-mini", "gpto3-mini", "gpt-o3-mini"},
			},
		},
		llms.LLMTypeClaude: {
			flag:         FlagAnthropicKey,
			envVar:       EnvAnthropicKey,
			defaultModel: string(anthropic.ModelClaudeSonnet4_0),
			fuzzyModelNames: map[string][]string{
				string(anthropic.ModelClaudeOpus4_1_20250805): {"opus", "opus4.1"},
				string(anthropic.ModelClaudeOpus4_0):          {"opus4"},
				string(anthropic.ModelClaudeSonnet4_0):        {"sonnet", "sonnet4"},
			},
		},
		llms.LLMTypeGrok: {
			flag:         FlagXAIKey,
			envVar:       EnvXAIKey,
			defaultModel: "grok-3",
			fuzzyModelNames: map[string][]string{
				"grok-2-1212":      {"2", "grok2", "grok-2"},
				"grok-3":           {"3", "grok3", "grok-3"},
				"grok-3-fast":      {"3-fast", "grok3-fast", "grok-3-fast"},
				"grok-3-mini":      {"3-mini", "grok3-mini", "grok-3-mini"},
				"grok-4-0709":      {"4", "grok4", "grok-4"},
				"grok-code-fast-1": {"code", "code-fast", "grok-code"},
			},
		},
	}

	if details, ok := mappings[provider]; ok {
		return details
	}

	return nil
}
