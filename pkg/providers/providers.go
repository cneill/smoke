// Package providers holds the built-in provider/model metadata used to configure an LLM session
// (default models, model aliases, context window sizes, and reasoning effort options).
package providers

import (
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/cneill/smoke/pkg/llms"
	"github.com/cneill/smoke/pkg/utils"
	"github.com/openai/openai-go/v3"
)

var (
	// ErrUnknownProvider is returned when the requested provider name is not recognized.
	ErrUnknownProvider = errors.New("unknown model provider")
	// ErrModelRequired is returned when a provider has no predefined models and none was specified.
	ErrModelRequired = errors.New("model must be specified explicitly")
	// ErrUnknownModel is returned when the requested model/alias does not match any known model.
	ErrUnknownModel = errors.New("unknown model")
	// ErrInvalidEffort is returned when the requested effort is not one of the provider's supported options.
	ErrInvalidEffort = errors.New("invalid effort")
)

// Model context sizes pulled from https://models.dev/models.json (see scripts/context_limits.txt).

// Models are listed in lookup order: when multiple models claim the same alias, the first one wins.
var chatGPT = llms.ModelInfos{ //nolint:gochecknoglobals
	{
		Provider:            llms.LLMTypeChatGPT,
		Model:               openai.ChatModelGPT5_1,
		Aliases:             []string{"5.1", "gpt5.1", "gpt-5.1"},
		ContextWindowTokens: 400_000,
	},
	{
		Provider:            llms.LLMTypeChatGPT,
		Model:               openai.ChatModelGPT5_2,
		Aliases:             []string{"5.2", "gpt5.2", "gpt-5.2"},
		ContextWindowTokens: 400_000,
	},
	{
		Provider:            llms.LLMTypeChatGPT,
		Model:               openai.ChatModelGPT5_2Pro,
		Aliases:             []string{"5.2-pro", "gpt5.2-pro", "gpt-5.2-pro"},
		ContextWindowTokens: 400_000,
	},
	{
		Provider:            llms.LLMTypeChatGPT,
		Model:               openai.ChatModelGPT5_4,
		Aliases:             []string{"5.4", "gpt5.4", "gpt-5.4"},
		ContextWindowTokens: 1_050_000,
	},
	{
		Provider:            llms.LLMTypeChatGPT,
		Model:               openai.ChatModelGPT5_4Mini,
		Aliases:             []string{"5.4-mini", "gpt5.4-mini", "gpt-5.4-mini"},
		ContextWindowTokens: 400_000,
	},
	{
		Provider:            llms.LLMTypeChatGPT,
		Model:               openai.ChatModelGPT5_4Nano,
		Aliases:             []string{"5.4-nano", "gpt5.4-nano", "gpt-5.4-nano"},
		ContextWindowTokens: 400_000,
	},
	{
		Provider:            llms.LLMTypeChatGPT,
		Model:               openai.ChatModelGPT5_5,
		Aliases:             []string{"5.5", "gpt5.5", "gpt-5.5"},
		ContextWindowTokens: 1_050_000,
	},
	{
		Provider:            llms.LLMTypeChatGPT,
		Model:               openai.ChatModelGPT5_6Sol,
		Aliases:             []string{"5.6", "5.6-sol", "gpt5.6"},
		ContextWindowTokens: 1_050_000,
	},
	{
		Provider:            llms.LLMTypeChatGPT,
		Model:               openai.ChatModelGPT5_6Terra,
		Aliases:             []string{"terra", "5.6-terra"},
		ContextWindowTokens: 1_050_000,
	},
	{
		Provider:            llms.LLMTypeChatGPT,
		Model:               openai.ChatModelGPT5_6Luna,
		Aliases:             []string{"5.6-luna"},
		ContextWindowTokens: 1_050_000,
	},
	{
		Provider:            llms.LLMTypeChatGPT,
		Model:               openai.ChatModelGPT6Astra,
		Aliases:             []string{"astra", "6", "6-astra"},
		ContextWindowTokens: 1_050_000,
	},
	{
		Provider:            llms.LLMTypeChatGPT,
		Model:               openai.ChatModelGPT6Sol,
		Aliases:             []string{"6-sol"},
		ContextWindowTokens: 1_050_000,
	},
	{
		Provider:            llms.LLMTypeChatGPT,
		Model:               openai.ChatModelGPT6Luna,
		Aliases:             []string{"luna", "6-luna"},
		ContextWindowTokens: 1_050_000,
	},
	{
		Provider:            llms.LLMTypeChatGPT,
		Model:               openai.ChatModelGPT6_1Sol,
		Aliases:             []string{"sol", "6.1-sol"},
		ContextWindowTokens: 1_050_000,
	},
}

var claude = llms.ModelInfos{ //nolint:gochecknoglobals
	{
		Provider:            llms.LLMTypeClaude,
		Model:               anthropic.ModelClaudeFable5_1,
		Aliases:             []string{"fable", "fable5.1", "f5.1"},
		ContextWindowTokens: 1_000_000,
	},
	{
		Provider:            llms.LLMTypeClaude,
		Model:               anthropic.ModelClaudeFable5,
		Aliases:             []string{"fable5", "f5"},
		ContextWindowTokens: 1_000_000,
	},
	{
		Provider:            llms.LLMTypeClaude,
		Model:               anthropic.ModelClaudeHaiku4_5,
		Aliases:             []string{"haiku4.5", "h45"},
		ContextWindowTokens: 200_000,
	},
	{
		Provider:            llms.LLMTypeClaude,
		Model:               anthropic.ModelClaudeHaiku5_5,
		Aliases:             []string{"haiku", "haiku5.5", "h55"},
		ContextWindowTokens: 1_000_000,
	},
	{
		Provider:            llms.LLMTypeClaude,
		Model:               anthropic.ModelClaudeOpus5_5,
		Aliases:             []string{"opus", "opus5.5", "o5.5"},
		ContextWindowTokens: 1_000_000,
	},
	{
		Provider:            llms.LLMTypeClaude,
		Model:               anthropic.ModelClaudeOpus5,
		Aliases:             []string{"opus5", "o5"},
		ContextWindowTokens: 1_000_000,
	},
	{
		Provider:            llms.LLMTypeClaude,
		Model:               anthropic.ModelClaudeOpus4_8,
		Aliases:             []string{"opus4.8", "o48"},
		ContextWindowTokens: 1_000_000,
	},
	{
		Provider:            llms.LLMTypeClaude,
		Model:               anthropic.ModelClaudeOpus4_7,
		Aliases:             []string{"opus4.7", "o47"},
		ContextWindowTokens: 1_000_000,
	},
	{
		Provider:            llms.LLMTypeClaude,
		Model:               anthropic.ModelClaudeOpus4_6,
		Aliases:             []string{"opus4.6", "o46"},
		ContextWindowTokens: 1_000_000,
	},
	{
		Provider:            llms.LLMTypeClaude,
		Model:               anthropic.ModelClaudeOpus4_5,
		Aliases:             []string{"opus4.5", "o45"},
		ContextWindowTokens: 200_000,
	},
	{
		Provider:            llms.LLMTypeClaude,
		Model:               anthropic.ModelClaudeSonnet5_5,
		Aliases:             []string{"sonnet", "sonnet5.5", "s5.5"},
		ContextWindowTokens: 1_000_000,
	},
	{
		Provider:            llms.LLMTypeClaude,
		Model:               anthropic.ModelClaudeSonnet5,
		Aliases:             []string{"sonnet5", "s5"},
		ContextWindowTokens: 1_000_000,
	},
	{
		Provider:            llms.LLMTypeClaude,
		Model:               anthropic.ModelClaudeSonnet4_6,
		Aliases:             []string{"sonnet4.6", "s46"},
		ContextWindowTokens: 1_000_000,
	},
}

var grok = llms.ModelInfos{ //nolint:gochecknoglobals
	{
		Provider:            llms.LLMTypeGrok,
		Model:               "grok-4.7",
		Aliases:             []string{"4.7", "470"},
		ContextWindowTokens: 500_000,
	},
	{
		Provider:            llms.LLMTypeGrok,
		Model:               "grok-4.6",
		Aliases:             []string{"4.6", "460"},
		ContextWindowTokens: 500_000,
	},
	{
		Provider:            llms.LLMTypeGrok,
		Model:               "grok-4.5",
		Aliases:             []string{"4.5", "450"},
		ContextWindowTokens: 500_000,
	},
	{
		Provider:            llms.LLMTypeGrok,
		Model:               "grok-4.3",
		Aliases:             []string{"4.3", "430"},
		ContextWindowTokens: 1_000_000,
	},
}

var registry = Registry{ //nolint:gochecknoglobals
	llms.LLMTypeChatGPT: {
		Provider:      llms.LLMTypeChatGPT,
		DefaultModel:  openai.ChatModelGPT6Sol,
		DefaultEffort: string(openai.ReasoningEffortMedium),
		EffortOptions: utils.ToStrings(
			openai.ReasoningEffortNone,
			openai.ReasoningEffortMinimal,
			openai.ReasoningEffortLow,
			openai.ReasoningEffortMedium,
			openai.ReasoningEffortHigh,
			openai.ReasoningEffortXhigh,
			openai.ReasoningEffortMax,
		),
		Models: chatGPT,
	},
	llms.LLMTypeClaude: {
		Provider:      llms.LLMTypeClaude,
		DefaultModel:  anthropic.ModelClaudeOpus5_5,
		DefaultEffort: string(anthropic.OutputConfigEffortMedium),
		EffortOptions: utils.ToStrings(
			anthropic.OutputConfigEffortLow,
			anthropic.OutputConfigEffortMedium,
			anthropic.OutputConfigEffortHigh,
			anthropic.OutputConfigEffortXhigh,
			anthropic.OutputConfigEffortMax,
		),
		Models: claude,
	},
	llms.LLMTypeGrok: {
		Provider:      llms.LLMTypeGrok,
		DefaultModel:  "grok-4.6",
		DefaultEffort: "medium",
		EffortOptions: []string{
			"none",
			"low",
			"medium",
			"high",
		},
		Models: grok,
	},
	llms.LLMTypeOllama: {
		// No default model; caller must specify one explicitly since Ollama models are user-installed.
		Provider:     llms.LLMTypeOllama,
		DefaultModel: "",
		Models:       nil,
	},
}

// All returns the built-in provider metadata.
func All() Registry {
	return registry
}

// Registry maps provider names (see llms.LLMType constants) to their Details.
type Registry map[string]*Details

// Names returns the sorted list of known provider names.
func (r Registry) Names() []string {
	names := slices.Collect(maps.Keys(r))
	slices.Sort(names)

	return names
}

// Details looks up the Details for the given provider name, returning ErrUnknownProvider if it is
// not recognized.
func (r Registry) Details(provider string) (*Details, error) {
	details, ok := r[provider]
	if !ok {
		return nil, fmt.Errorf("%w: %q, must choose one of %s", ErrUnknownProvider, provider, strings.Join(r.Names(), ", "))
	}

	return details, nil
}

// Details holds the metadata needed to configure a given LLM provider: its name, default model, default
// reasoning effort, valid effort options, and known models/aliases.
type Details struct {
	Provider      llms.LLMType
	DefaultModel  string
	DefaultEffort string
	EffortOptions []string
	Models        llms.ModelInfos
}

// ModelInfo resolves the given search string (a canonical model name or alias) to its ModelInfo. If
// search is empty, the provider's default model is returned. If the provider has no predefined models
// (e.g. Ollama), a ModelInfo with only Provider and Model (set to search) is returned, and
// ErrModelRequired is returned if search is empty.
func (d Details) ModelInfo(search string) (llms.ModelInfo, error) {
	if len(d.Models) == 0 {
		if search == "" {
			return llms.ModelInfo{}, ErrModelRequired
		}

		return llms.ModelInfo{Provider: d.Provider, Model: search}, nil
	}

	if search == "" {
		search = d.DefaultModel
	}

	// Exact model names take precedence over aliases.
	if idx := slices.IndexFunc(d.Models, func(info llms.ModelInfo) bool { return info.Model == search }); idx >= 0 {
		return d.Models[idx], nil
	}

	// Aliases are matched in declaration order, so the first model claiming an alias wins.
	if idx := slices.IndexFunc(d.Models, func(info llms.ModelInfo) bool {
		return slices.Contains(info.Aliases, search)
	}); idx >= 0 {
		return d.Models[idx], nil
	}

	return llms.ModelInfo{}, fmt.Errorf("%w: %q\n\n%s", ErrUnknownModel, search, d.Models)
}

// Effort validates the given effort string against the provider's supported EffortOptions. If effort
// is empty, the provider's default effort is returned.
func (d Details) Effort(effort string) (string, error) {
	if effort == "" {
		return d.DefaultEffort, nil
	}

	if !slices.Contains(d.EffortOptions, effort) {
		return "", fmt.Errorf("%w: options are %s", ErrInvalidEffort, strings.Join(d.EffortOptions, ", "))
	}

	return effort, nil
}
