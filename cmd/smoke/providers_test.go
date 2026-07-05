package main

import (
	"testing"

	"github.com/cneill/smoke/pkg/llms"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProviderDetailsGetModelInfo(t *testing.T) {
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
			wantModel:         "gpt-5.4",
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

			details, err := getProviders().details(test.provider)
			require.NoError(t, err)

			model, info, err := details.getModelInfo(test.search)
			require.NoError(t, err)
			assert.Equal(t, test.wantModel, model)
			assert.Equal(t, test.wantContextTokens, info.contextWindowTokens)
		})
	}
}

func TestProviderDetailsGetModelInfoUnknownModel(t *testing.T) {
	t.Parallel()

	details, err := getProviders().details(llms.LLMTypeChatGPT)
	require.NoError(t, err)

	_, _, err = details.getModelInfo("bogus")
	require.Error(t, err)
	require.ErrorContains(t, err, "unknown model")
	require.ErrorContains(t, err, "Model aliases")
	require.ErrorContains(t, err, "gpt-5.4: 5.4, gpt5.4, gpt-5.4")
}

func TestProviderDetailsGetModelInfoPassesThroughModelsForOllama(t *testing.T) {
	t.Parallel()

	details, err := getProviders().details(llms.LLMTypeOllama)
	require.NoError(t, err)

	model, info, err := details.getModelInfo("llama3.1")
	require.NoError(t, err)
	assert.Equal(t, "llama3.1", model)
	assert.Zero(t, info)
}
