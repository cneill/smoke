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

// Model context sizes pulled from https://models.dev/models.json
var registry = Registry{ //nolint:gochecknoglobals
	llms.LLMTypeChatGPT: {
		DefaultModel:  "gpt-5.5",
		DefaultEffort: string(openai.ReasoningEffortMedium),
		EffortOptions: utils.ToStrings(
			openai.ReasoningEffortNone,
			openai.ReasoningEffortMinimal,
			openai.ReasoningEffortLow,
			openai.ReasoningEffortMedium,
			openai.ReasoningEffortHigh,
			openai.ReasoningEffortXhigh,
		),
		Models: ModelInfos{
			openai.ChatModelGPT4o:       {Aliases: []string{"4o", "gpt4o", "gpt-4o"}, ContextWindowTokens: 128000},
			openai.ChatModelGPT4oMini:   {Aliases: []string{"4o-mini", "gpt4o-mini", "gpt-4o-mini"}, ContextWindowTokens: 128000},
			openai.ChatModelGPT4_1:      {Aliases: []string{"4.1", "gpt4.1", "gpt-4.1"}, ContextWindowTokens: 1047576},
			openai.ChatModelGPT4_1Mini:  {Aliases: []string{"4.1-mini", "gpt4.1-mini", "gpt-4.1-mini"}, ContextWindowTokens: 1047576},
			openai.ChatModelGPT4_1Nano:  {Aliases: []string{"4.1-nano", "gpt4.1-nano", "gpt-4.1-nano"}, ContextWindowTokens: 1047576},
			openai.ChatModelGPT5:        {Aliases: []string{"5", "gpt5", "gpt-5"}, ContextWindowTokens: 400000},
			openai.ChatModelGPT5Mini:    {Aliases: []string{"5-mini", "gpt5-mini", "gpt-5-mini"}, ContextWindowTokens: 400000},
			openai.ChatModelGPT5Nano:    {Aliases: []string{"5-nano", "gpt5-nano", "gpt-5-nano"}, ContextWindowTokens: 400000},
			openai.ChatModelGPT5_1:      {Aliases: []string{"5.1", "gpt5.1", "gpt-5.1"}, ContextWindowTokens: 400000},
			openai.ChatModelGPT5_1Mini:  {Aliases: []string{"5.1-mini", "gpt5.1-mini", "gpt-5.1-mini"}, ContextWindowTokens: 400000},
			openai.ChatModelGPT5_1Codex: {Aliases: []string{"5.1-codex", "gpt5.1-codex", "gpt-5.1-codex"}, ContextWindowTokens: 400000},
			openai.ChatModelGPT5_2:      {Aliases: []string{"5.2", "gpt5.2", "gpt-5.2"}, ContextWindowTokens: 400000},
			openai.ChatModelGPT5_2Pro:   {Aliases: []string{"5.2-pro", "gpt5.2-pro", "gpt-5.2-pro"}, ContextWindowTokens: 400000},
			openai.ChatModelGPT5_4:      {Aliases: []string{"5.4", "gpt5.4", "gpt-5.4"}, ContextWindowTokens: 1050000},
			openai.ChatModelGPT5_4Mini:  {Aliases: []string{"5.4-mini", "gpt5.4-mini", "gpt-5.4-mini"}, ContextWindowTokens: 400000},
			openai.ChatModelGPT5_4Nano:  {Aliases: []string{"5.4-nano", "gpt5.4-nano", "gpt-5.4-nano"}, ContextWindowTokens: 400000},
			// Some day OpenAI will update their SDK...
			"gpt-5.5":       {Aliases: []string{"5.5", "gpt5.5", "gpt-5.5"}, ContextWindowTokens: 1050000},
			"gpt-5.6-sol":   {Aliases: []string{"5.6", "sol", "5.6-sol", "gpt5.6"}, ContextWindowTokens: 1050000},
			"gpt-5.6-terra": {Aliases: []string{"terra", "5.6-terra"}, ContextWindowTokens: 1050000},
			"gpt-5.6-luna":  {Aliases: []string{"luna", "5.6-luna"}, ContextWindowTokens: 1050000},
		},
	},
	llms.LLMTypeClaude: {
		DefaultModel:  anthropic.ModelClaudeSonnet5,
		DefaultEffort: string(anthropic.OutputConfigEffortMedium),
		EffortOptions: utils.ToStrings(
			anthropic.OutputConfigEffortLow,
			anthropic.OutputConfigEffortMedium,
			anthropic.OutputConfigEffortHigh,
			anthropic.OutputConfigEffortXhigh,
			anthropic.OutputConfigEffortMax,
		),
		Models: ModelInfos{
			anthropic.ModelClaudeFable5:    {Aliases: []string{"fable", "fable5"}, ContextWindowTokens: 1000000},
			anthropic.ModelClaudeOpus4_8:   {Aliases: []string{"opus", "opus4.8", "o48"}, ContextWindowTokens: 1000000},
			anthropic.ModelClaudeOpus4_7:   {Aliases: []string{"opus4.7", "o47"}, ContextWindowTokens: 1000000},
			anthropic.ModelClaudeOpus4_6:   {Aliases: []string{"opus4.6", "o46"}, ContextWindowTokens: 1000000},
			anthropic.ModelClaudeOpus4_5:   {Aliases: []string{"opus4.5", "o45"}, ContextWindowTokens: 200000},
			anthropic.ModelClaudeSonnet5:   {Aliases: []string{"sonnet", "sonnet5", "s5"}, ContextWindowTokens: 1000000},
			anthropic.ModelClaudeSonnet4_6: {Aliases: []string{"sonnet4.6", "s46"}, ContextWindowTokens: 1000000},
			anthropic.ModelClaudeSonnet4_5: {Aliases: []string{"sonnet4.5", "s45"}, ContextWindowTokens: 200000},
			anthropic.ModelClaudeHaiku4_5:  {Aliases: []string{"haiku", "haiku4.5", "h45"}, ContextWindowTokens: 200000},
		},
	},
	llms.LLMTypeGrok: {
		DefaultModel:  "grok-build-0.1",
		DefaultEffort: "medium",
		EffortOptions: []string{
			"none",
			"low",
			"medium",
			"high",
		},
		Models: ModelInfos{
			"grok-4.5":                 {Aliases: []string{"4.5", "450"}, ContextWindowTokens: 500000},
			"grok-build-0.1":           {Aliases: []string{"build", "fast"}, ContextWindowTokens: 256000},
			"grok-4.3":                 {Aliases: []string{"4.3", "430"}, ContextWindowTokens: 1000000},
			"grok-4.20-0309-reasoning": {Aliases: []string{"4.2", "420"}, ContextWindowTokens: 1000000},
		},
	},
	llms.LLMTypeOllama: {
		// No default model; caller must specify one explicitly since Ollama models are user-installed.
		DefaultModel: "",
		Models:       ModelInfos{},
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

// Details holds the metadata needed to configure a given LLM provider: its default model, default
// reasoning effort, valid effort options, and known models/aliases.
type Details struct {
	DefaultModel  string
	DefaultEffort string
	EffortOptions []string
	Models        ModelInfos
}

// ModelInfo resolves the given search string (a canonical model name or alias) to a canonical model
// name and its ModelInfo. If search is empty, the provider's default model is returned. If the
// provider has no predefined models (e.g. Ollama), search is returned verbatim, and ErrModelRequired
// is returned if search is empty.
func (d Details) ModelInfo(search string) (string, ModelInfo, error) {
	if len(d.Models) == 0 {
		if search == "" {
			return "", ModelInfo{}, ErrModelRequired
		}

		return search, ModelInfo{}, nil
	}

	if search == "" {
		return d.DefaultModel, d.Models[d.DefaultModel], nil
	}

	for model, info := range d.Models {
		if model == search {
			return model, info, nil
		}

		if slices.Contains(info.Aliases, search) {
			return model, info, nil
		}
	}

	return "", ModelInfo{}, fmt.Errorf("%w: %q\n\n%s", ErrUnknownModel, search, d.Models)
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

// ModelInfo describes a single model: its known aliases and context window size.
type ModelInfo struct {
	Aliases             []string
	ContextWindowTokens int64
}

// ModelInfos maps canonical model names to their ModelInfo.
type ModelInfos map[string]ModelInfo

func (m ModelInfos) String() string {
	sb := strings.Builder{}
	sb.Grow(64)
	sb.WriteString("Model aliases:\n")

	modelNames := slices.Collect(maps.Keys(m))
	slices.Sort(modelNames)

	for _, modelName := range modelNames {
		sb.WriteString(modelName)
		sb.WriteString(": ")
		sb.WriteString(strings.Join(m[modelName].Aliases, ", "))
		sb.WriteByte('\n')
	}

	return sb.String()
}
