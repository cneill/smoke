package providers_test

import (
	"testing"

	"github.com/cneill/smoke/pkg/llms"
	"github.com/cneill/smoke/pkg/providers"
	"github.com/openai/openai-go/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDetailsModelInfo(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name              string
		provider          string
		search            string
		wantModel         string
		wantContextTokens int64
	}{
		{
			name:              "default model",
			provider:          llms.LLMTypeChatGPT,
			search:            "",
			wantModel:         openai.ChatModelGPT6Sol,
			wantContextTokens: 1_050_000,
		},
		{
			name:              "canonical model",
			provider:          llms.LLMTypeClaude,
			search:            "claude-sonnet-4-6",
			wantModel:         "claude-sonnet-4-6",
			wantContextTokens: 1_000_000,
		},
		{
			name:              "alias",
			provider:          llms.LLMTypeGrok,
			search:            "470",
			wantModel:         "grok-4.7",
			wantContextTokens: 500_000,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			details, err := providers.All().Details(test.provider)
			require.NoError(t, err)

			info, err := details.ModelInfo(test.search)
			require.NoError(t, err)
			assert.Equal(t, test.provider, string(info.Provider))
			assert.Equal(t, test.wantModel, info.Model)
			assert.Equal(t, test.wantContextTokens, info.ContextWindowTokens)
		})
	}
}

func TestDetailsModelInfoUnknownModel(t *testing.T) {
	t.Parallel()

	details, err := providers.All().Details(llms.LLMTypeChatGPT)
	require.NoError(t, err)

	_, err = details.ModelInfo("bogus")
	require.Error(t, err)
	require.ErrorIs(t, err, providers.ErrUnknownModel)
	require.ErrorContains(t, err, "Model aliases")
	require.ErrorContains(t, err, "gpt-5.4: 5.4, gpt5.4, gpt-5.4")
}

func TestDetailsModelInfoPassesThroughModelsForOllama(t *testing.T) {
	t.Parallel()

	details, err := providers.All().Details(llms.LLMTypeOllama)
	require.NoError(t, err)

	info, err := details.ModelInfo("llama3.1")
	require.NoError(t, err)
	assert.Equal(t, llms.ModelInfo{Provider: llms.LLMTypeOllama, Model: "llama3.1"}, info)
}

func TestDetailsModelInfoRequiresModelForOllama(t *testing.T) {
	t.Parallel()

	details, err := providers.All().Details(llms.LLMTypeOllama)
	require.NoError(t, err)

	_, err = details.ModelInfo("")
	require.Error(t, err)
	require.ErrorIs(t, err, providers.ErrModelRequired)
}

func TestRegistryDetailsUnknownProvider(t *testing.T) {
	t.Parallel()

	_, err := providers.All().Details("bogus")
	require.Error(t, err)
	require.ErrorIs(t, err, providers.ErrUnknownProvider)
}
