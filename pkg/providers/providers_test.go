package providers_test

import (
	"testing"

	"github.com/cneill/smoke/pkg/llms"
	"github.com/cneill/smoke/pkg/providers"
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
			wantModel:         "gpt-5.5",
			wantContextTokens: 1050000,
		},
		{
			name:              "canonical model",
			provider:          llms.LLMTypeClaude,
			search:            "claude-sonnet-4-6",
			wantModel:         "claude-sonnet-4-6",
			wantContextTokens: 1000000,
		},
		{
			name:              "alias",
			provider:          llms.LLMTypeGrok,
			search:            "build",
			wantModel:         "grok-build-0.1",
			wantContextTokens: 256000,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			details, err := providers.All().Details(test.provider)
			require.NoError(t, err)

			model, info, err := details.ModelInfo(test.search)
			require.NoError(t, err)
			assert.Equal(t, test.wantModel, model)
			assert.Equal(t, test.wantContextTokens, info.ContextWindowTokens)
		})
	}
}

func TestDetailsModelInfoUnknownModel(t *testing.T) {
	t.Parallel()

	details, err := providers.All().Details(llms.LLMTypeChatGPT)
	require.NoError(t, err)

	_, _, err = details.ModelInfo("bogus")
	require.Error(t, err)
	require.ErrorIs(t, err, providers.ErrUnknownModel)
	require.ErrorContains(t, err, "Model aliases")
	require.ErrorContains(t, err, "gpt-5.4: 5.4, gpt5.4, gpt-5.4")
}

func TestDetailsModelInfoPassesThroughModelsForOllama(t *testing.T) {
	t.Parallel()

	details, err := providers.All().Details(llms.LLMTypeOllama)
	require.NoError(t, err)

	model, info, err := details.ModelInfo("llama3.1")
	require.NoError(t, err)
	assert.Equal(t, "llama3.1", model)
	assert.Zero(t, info)
}

func TestDetailsModelInfoRequiresModelForOllama(t *testing.T) {
	t.Parallel()

	details, err := providers.All().Details(llms.LLMTypeOllama)
	require.NoError(t, err)

	_, _, err = details.ModelInfo("")
	require.Error(t, err)
	require.ErrorIs(t, err, providers.ErrModelRequired)
}

func TestRegistryDetailsUnknownProvider(t *testing.T) {
	t.Parallel()

	_, err := providers.All().Details("bogus")
	require.Error(t, err)
	require.ErrorIs(t, err, providers.ErrUnknownProvider)
}
