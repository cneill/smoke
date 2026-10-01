package statusline_test

import (
	"testing"

	"github.com/cneill/smoke/pkg/llms"
	"github.com/cneill/smoke/pkg/models/statusline"
	"github.com/cneill/smoke/pkg/smoke"
	"github.com/stretchr/testify/assert"
)

func TestViewContextUsage(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		contextSize  int64
		wantContains []string
		wantAbsent   []string
	}{
		{
			name:         "known context size",
			contextSize:  1000,
			wantContains: []string{"chatgpt/test-model", "ctx: 500", "/ 1,000", "(50.00%)"},
		},
		{
			name:         "unknown context size",
			contextSize:  0,
			wantContains: []string{"chatgpt/test-model", "ctx: 500"},
			wantAbsent:   []string{" / ", "NaN", "Inf", "%"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			model := statusline.New(200, llms.ModelInfo{
				Provider:            llms.LLMTypeChatGPT,
				Model:               "test-model",
				ContextWindowTokens: test.contextSize,
			})
			model, _ = model.Update(smoke.UsageUpdateMessage{ContextWindowTokens: 500})

			view := model.View()
			for _, want := range test.wantContains {
				assert.Contains(t, view, want)
			}

			for _, absent := range test.wantAbsent {
				assert.NotContains(t, view, absent)
			}
		})
	}
}
