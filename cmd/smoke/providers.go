package main

import (
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/cneill/smoke/pkg/llms"
	"github.com/cneill/smoke/pkg/utils"
	"github.com/openai/openai-go/v3"
)

var providers = providerMappings{ //nolint:gochecknoglobals
	llms.LLMTypeChatGPT: {
		apiKeyFlag:    FlagOpenAIKey,
		apiKeyEnvVar:  EnvOpenAIKey,
		defaultModel:  openai.ChatModelGPT5_4,
		defaultEffort: string(openai.ReasoningEffortMedium),
		effortOptions: utils.ToStrings(
			openai.ReasoningEffortNone,
			openai.ReasoningEffortMinimal,
			openai.ReasoningEffortLow,
			openai.ReasoningEffortMedium,
			openai.ReasoningEffortHigh,
			openai.ReasoningEffortXhigh,
		),
		models: modelInfos{
			openai.ChatModelGPT4o:       {aliases: []string{"4o", "gpt4o", "gpt-4o"}, contextWindowTokens: 128000},
			openai.ChatModelGPT4oMini:   {aliases: []string{"4o-mini", "gpt4o-mini", "gpt-4o-mini"}, contextWindowTokens: 128000},
			openai.ChatModelGPT4_1:      {aliases: []string{"4.1", "gpt4.1", "gpt-4.1"}, contextWindowTokens: 1047576},
			openai.ChatModelGPT4_1Mini:  {aliases: []string{"4.1-mini", "gpt4.1-mini", "gpt-4.1-mini"}, contextWindowTokens: 1047576},
			openai.ChatModelGPT4_1Nano:  {aliases: []string{"4.1-nano", "gpt4.1-nano", "gpt-4.1-nano"}, contextWindowTokens: 1047576},
			openai.ChatModelGPT5:        {aliases: []string{"5", "gpt5", "gpt-5"}, contextWindowTokens: 400000},
			openai.ChatModelGPT5Mini:    {aliases: []string{"5-mini", "gpt5-mini", "gpt-5-mini"}, contextWindowTokens: 400000},
			openai.ChatModelGPT5Nano:    {aliases: []string{"5-nano", "gpt5-nano", "gpt-5-nano"}, contextWindowTokens: 400000},
			openai.ChatModelGPT5_1:      {aliases: []string{"5.1", "gpt5.1", "gpt-5.1"}, contextWindowTokens: 400000},
			openai.ChatModelGPT5_1Mini:  {aliases: []string{"5.1-mini", "gpt5.1-mini", "gpt-5.1-mini"}, contextWindowTokens: 400000},
			openai.ChatModelGPT5_1Codex: {aliases: []string{"5.1-codex", "gpt5.1-codex", "gpt-5.1-codex"}, contextWindowTokens: 400000},
			openai.ChatModelGPT5_2:      {aliases: []string{"5.2", "gpt5.2", "gpt-5.2"}, contextWindowTokens: 400000},
			openai.ChatModelGPT5_2Pro:   {aliases: []string{"5.2-pro", "gpt5.2-pro", "gpt-5.2-pro"}, contextWindowTokens: 400000},
			openai.ChatModelGPT5_4:      {aliases: []string{"5.4", "gpt5.4", "gpt-5.4"}, contextWindowTokens: 1050000},
			openai.ChatModelGPT5_4Mini:  {aliases: []string{"5.4-mini", "gpt5.4-mini", "gpt-5.4-mini"}, contextWindowTokens: 400000},
			openai.ChatModelGPT5_4Nano:  {aliases: []string{"5.4-nano", "gpt5.4-nano", "gpt-5.4-nano"}, contextWindowTokens: 400000},
			// Some day OpenAI will update their SDK...
			"gpt-5.5": {aliases: []string{"5.5", "gpt5.5", "gpt-5.5"}, contextWindowTokens: 1050000},
		},
	},
	llms.LLMTypeClaude: {
		apiKeyFlag:    FlagAnthropicKey,
		apiKeyEnvVar:  EnvAnthropicKey,
		defaultModel:  anthropic.ModelClaudeSonnet4_6,
		defaultEffort: string(anthropic.OutputConfigEffortMedium),
		effortOptions: utils.ToStrings(
			anthropic.OutputConfigEffortLow,
			anthropic.OutputConfigEffortMedium,
			anthropic.OutputConfigEffortHigh,
			anthropic.OutputConfigEffortXhigh,
			anthropic.OutputConfigEffortMax,
		),
		models: modelInfos{
			anthropic.ModelClaudeFable5:    {aliases: []string{"fable", "fable5"}, contextWindowTokens: 1000000},
			anthropic.ModelClaudeOpus4_8:   {aliases: []string{"opus", "opus4.8", "o48"}, contextWindowTokens: 1000000},
			anthropic.ModelClaudeOpus4_7:   {aliases: []string{"opus4.7", "o47"}, contextWindowTokens: 1000000},
			anthropic.ModelClaudeOpus4_6:   {aliases: []string{"opus4.6", "o46"}, contextWindowTokens: 1000000},
			anthropic.ModelClaudeOpus4_5:   {aliases: []string{"opus4.5", "o45"}, contextWindowTokens: 200000},
			anthropic.ModelClaudeSonnet5:   {aliases: []string{"sonnet", "sonnet5", "s5"}, contextWindowTokens: 1000000},
			anthropic.ModelClaudeSonnet4_6: {aliases: []string{"sonnet4.6", "s46"}, contextWindowTokens: 1000000},
			anthropic.ModelClaudeSonnet4_5: {aliases: []string{"sonnet4.5", "s45"}, contextWindowTokens: 200000},
			anthropic.ModelClaudeHaiku4_5:  {aliases: []string{"haiku", "haiku4.5", "h45"}, contextWindowTokens: 200000},
		},
	},
	llms.LLMTypeGrok: {
		apiKeyFlag:    FlagXAIKey,
		apiKeyEnvVar:  EnvXAIKey,
		defaultModel:  "grok-build-0.1",
		defaultEffort: "medium",
		effortOptions: []string{
			"none",
			"low",
			"medium",
			"high",
		},
		models: modelInfos{
			"grok-build-0.1":           {aliases: []string{"build", "fast"}, contextWindowTokens: 256000},
			"grok-4.3":                 {aliases: []string{"4.3", "430"}, contextWindowTokens: 1000000},
			"grok-4.20-0309-reasoning": {aliases: []string{"4.2", "420"}, contextWindowTokens: 1000000},
		},
	},
	llms.LLMTypeOllama: {
		// No API key required; model must be specified explicitly via --model.
		apiKeyFlag:   "",
		apiKeyEnvVar: "",
		baseURLFlag:  FlagOllamaHost,
		defaultModel: "",
		models:       modelInfos{},
	},
}

// getProviders returns the built-in provider metadata.
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
	apiKeyFlag    string
	apiKeyEnvVar  string
	baseURLFlag   string
	defaultModel  string
	defaultEffort string
	effortOptions []string
	models        modelInfos
}

func (p providerDetails) getModelInfo(search string) (string, modelInfo, error) {
	// Provider has no predefined models (e.g. ollama): pass through verbatim.
	if len(p.models) == 0 {
		if search == "" {
			return "", modelInfo{}, fmt.Errorf("this provider requires --model to be specified explicitly")
		}

		return search, modelInfo{}, nil
	}

	if search == "" {
		return p.defaultModel, p.models[p.defaultModel], nil
	}

	for model, info := range p.models {
		if model == search {
			return model, info, nil
		}

		if slices.Contains(info.aliases, search) {
			return model, info, nil
		}
	}

	return "", modelInfo{}, fmt.Errorf("unknown model: %q\n\n%s", search, p.models)
}

func (p providerDetails) getEffort(effort string) (string, error) {
	if effort == "" {
		return p.defaultEffort, nil
	}

	if !slices.Contains(p.effortOptions, effort) {
		return "", fmt.Errorf("effort options: %s", strings.Join(p.effortOptions, ", "))
	}

	return effort, nil
}

type modelInfo struct {
	aliases             []string
	contextWindowTokens int64
}

type modelInfos map[string]modelInfo

func (m modelInfos) String() string {
	builder := strings.Builder{}
	builder.Grow(64)
	builder.WriteString("Model aliases:\n")

	modelNames := slices.Collect(maps.Keys(m))
	slices.Sort(modelNames)

	for _, modelName := range modelNames {
		builder.WriteString(modelName)
		builder.WriteString(": ")
		builder.WriteString(strings.Join(m[modelName].aliases, ", "))
		builder.WriteByte('\n')
	}

	return builder.String()
}
