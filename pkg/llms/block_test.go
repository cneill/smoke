package llms_test

import (
	"encoding/json"
	"testing"

	"github.com/cneill/smoke/pkg/llms"
	"github.com/cneill/smoke/pkg/tools"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBlockOK(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		block   llms.Block
		wantErr string
	}{
		{
			name:  "text",
			block: llms.TextContentBlock("hello"),
		},
		{
			name:    "no payload",
			block:   llms.Block{},
			wantErr: "exactly one payload",
		},
		{
			name: "multiple payloads",
			block: llms.Block{
				Text:  &llms.TextBlock{},
				Image: &llms.ImageBlock{},
			},
			wantErr: "exactly one payload",
		},
		{
			name:    "image missing media type",
			block:   llms.ImageContentBlock([]byte{1}, ""),
			wantErr: "missing media type",
		},
		{
			name:    "invalid OpenAI source",
			block:   reasoningBlock(llms.ReasoningDetail{OpenAI: &llms.OpenAIReasoningDetail{Source: llms.LLMTypeClaude}}),
			wantErr: "invalid OpenAI reasoning source",
		},
		{
			name: "invalid Anthropic source",
			block: reasoningBlock(llms.ReasoningDetail{
				Anthropic: &llms.AnthropicReasoningDetail{Source: llms.LLMTypeChatGPT},
			}),
			wantErr: "invalid Anthropic reasoning source",
		},
		{
			name:  "valid Gemini",
			block: reasoningBlock(llms.ReasoningDetail{Gemini: &llms.GeminiReasoningDetail{Source: llms.LLMTypeGemini}}),
		},
		{
			name:    "invalid Gemini source",
			block:   reasoningBlock(llms.ReasoningDetail{Gemini: &llms.GeminiReasoningDetail{Source: llms.LLMTypeClaude}}),
			wantErr: "invalid Gemini reasoning source",
		},
		{
			name: "multiple reasoning details",
			block: reasoningBlock(llms.ReasoningDetail{
				Anthropic: &llms.AnthropicReasoningDetail{Source: llms.LLMTypeClaude},
				Gemini:    &llms.GeminiReasoningDetail{Source: llms.LLMTypeGemini},
			}),
			wantErr: "must supply exactly one of OpenAI, Anthropic, or Gemini",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			err := test.block.OK()
			if test.wantErr == "" {
				require.NoError(t, err)

				return
			}

			require.ErrorContains(t, err, test.wantErr)
		})
	}
}

func TestBlockTypeIsDerivedFromPayload(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		block llms.Block
		want  llms.BlockType
	}{
		{name: "empty", block: llms.Block{}, want: llms.BlockTypeUnknown},
		{name: "text", block: llms.Block{Text: &llms.TextBlock{}}, want: llms.BlockTypeText},
		{name: "image", block: llms.Block{Image: &llms.ImageBlock{}}, want: llms.BlockTypeImage},
		{name: "tool call", block: llms.Block{ToolCall: &llms.ToolCallBlock{}}, want: llms.BlockTypeToolCall},
		{name: "reasoning", block: llms.Block{Reasoning: &llms.ReasoningBlock{}}, want: llms.BlockTypeReasoning},
		{
			name:  "ambiguous",
			block: llms.Block{Text: &llms.TextBlock{}, Image: &llms.ImageBlock{}},
			want:  llms.BlockTypeUnknown,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, test.want, test.block.Type())
		})
	}
}

// reasoningBlock builds a reasoning block without validation or cloning, so invalid details can be tested.
func reasoningBlock(detail llms.ReasoningDetail) llms.Block {
	return llms.Block{Reasoning: &llms.ReasoningBlock{Detail: detail}}
}

func TestReasoningDetailType(t *testing.T) {
	t.Parallel()

	openAI := &llms.OpenAIReasoningDetail{Source: llms.LLMTypeChatGPT}
	anthropic := &llms.AnthropicReasoningDetail{Source: llms.LLMTypeClaude}
	gemini := &llms.GeminiReasoningDetail{Source: llms.LLMTypeGemini}

	tests := []struct {
		name   string
		detail llms.ReasoningDetail
		want   llms.ReasoningDetailType
	}{
		{name: "empty", detail: llms.ReasoningDetail{}, want: llms.ReasoningDetailTypeUnknown},
		{name: "openai", detail: llms.ReasoningDetail{OpenAI: openAI}, want: llms.ReasoningDetailTypeOpenAI},
		{name: "anthropic", detail: llms.ReasoningDetail{Anthropic: anthropic}, want: llms.ReasoningDetailTypeAnthropic},
		{name: "gemini", detail: llms.ReasoningDetail{Gemini: gemini}, want: llms.ReasoningDetailTypeGemini},
		{
			name:   "gemini and anthropic",
			detail: llms.ReasoningDetail{Anthropic: anthropic, Gemini: gemini},
			want:   llms.ReasoningDetailTypeUnknown,
		},
		{
			name:   "all three",
			detail: llms.ReasoningDetail{OpenAI: openAI, Anthropic: anthropic, Gemini: gemini},
			want:   llms.ReasoningDetailTypeUnknown,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, test.want, test.detail.Type())
		})
	}
}

func TestGeminiReasoningBlockRoundTrip(t *testing.T) {
	t.Parallel()

	block := llms.ReasoningContentBlock(llms.ReasoningBlock{
		Summary: []string{"thinking"},
		Detail: llms.ReasoningDetail{
			Gemini: &llms.GeminiReasoningDetail{Source: llms.LLMTypeGemini, Signature: "sig"},
		},
	})

	encoded, err := json.Marshal(block)
	require.NoError(t, err)

	var decoded llms.Block
	require.NoError(t, json.Unmarshal(encoded, &decoded))
	require.NoError(t, decoded.OK())
	assert.Equal(t, block, decoded)

	clone := block.Clone()
	clone.Reasoning.Detail.Gemini.Signature = "changed"

	assert.Equal(t, "sig", block.Reasoning.Detail.Gemini.Signature)
	assert.Equal(t, []string{"thinking"}, decoded.Reasoning.Summary)
}

func TestBlockJSONTypeIsDerivedAndValidated(t *testing.T) {
	t.Parallel()

	encoded, err := json.Marshal(llms.TextContentBlock("hello"))
	require.NoError(t, err)
	assert.JSONEq(t, `{"type":"text","text":{"text":"hello"}}`, string(encoded))

	var decoded llms.Block
	require.NoError(t, json.Unmarshal(encoded, &decoded))
	assert.Equal(t, llms.BlockTypeText, decoded.Type())
	assert.Equal(t, "hello", decoded.Text.Text)

	err = json.Unmarshal([]byte(`{"type":"image","text":{"text":"hello"}}`), &decoded)
	require.ErrorContains(t, err, "does not match payload type")
}

func TestMessageProjectionsPreserveOrder(t *testing.T) {
	t.Parallel()

	call := llms.ToolCall{ID: "call", Name: "tool", Args: tools.Args{"n": json.Number("1")}}
	msg := llms.NewMessage(
		llms.WithRole(llms.RoleAssistant),
		llms.WithBlocks(
			llms.TextContentBlock("first"),
			llms.ImageContentBlock([]byte{1, 2}, "image/jpeg"),
			llms.ToolContentBlock(call),
			llms.TextContentBlock("second"),
			llms.ReasoningContentBlock(llms.ReasoningBlock{
				Summary: []string{"summary"},
				Detail: llms.ReasoningDetail{
					OpenAI: &llms.OpenAIReasoningDetail{Source: llms.LLMTypeChatGPT},
				},
			}),
		),
	)

	assert.Equal(t, "firstsecond", msg.TextContent())
	assert.Equal(t, []llms.ImageBlock{{Data: []byte{1, 2}, MediaType: "image/jpeg"}}, msg.Images())
	assert.Equal(t, llms.ToolCalls{call}, msg.ToolCalls())
	assert.Equal(t, []string{"summary"}, msg.ReasoningSummaries())
	assert.Equal(t, "data:image/jpeg;base64,AQI=", msg.ImageB64URL())
}

func TestMessageCloneIsDeep(t *testing.T) {
	t.Parallel()

	msg := llms.NewMessage(
		llms.WithRole(llms.RoleAssistant),
		llms.WithBlocks(
			llms.ImageContentBlock([]byte{1}, "image/png"),
			llms.ToolContentBlock(
				llms.ToolCall{
					Args: tools.Args{
						"nested": map[string]any{"value": json.Number("2")},
					},
				}),
			llms.ReasoningContentBlock(llms.ReasoningBlock{
				Summary: []string{"visible"},
				Detail: llms.ReasoningDetail{
					Anthropic: &llms.AnthropicReasoningDetail{
						Source: llms.LLMTypeClaude,
						Data:   "secret",
					},
				},
			}),
		))
	clone := msg.Clone()

	clone.Blocks[0].Image.Data[0] = 9
	nested, ok := clone.Blocks[1].ToolCall.ToolCall.Args["nested"].(map[string]any)
	require.True(t, ok)

	nested["value"] = json.Number("3")
	clone.Blocks[2].Reasoning.Summary[0] = "changed"
	clone.Blocks[2].Reasoning.Detail.Anthropic.Data = "changed"

	assert.Equal(t, byte(1), msg.Blocks[0].Image.Data[0])
	nested, ok = msg.Blocks[1].ToolCall.ToolCall.Args["nested"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, json.Number("2"), nested["value"])
	assert.Equal(t, "visible", msg.Blocks[2].Reasoning.Summary[0])
	assert.Equal(t, "secret", msg.Blocks[2].Reasoning.Detail.Anthropic.Data)
}

func TestMessageReasoningOpaqueDataIsNotRendered(t *testing.T) {
	t.Parallel()

	const secret = "opaque-secret"

	msg := llms.NewMessage(
		llms.WithRole(llms.RoleAssistant),
		llms.WithBlocks(
			llms.TextContentBlock("answer"),
			llms.ReasoningContentBlock(llms.ReasoningBlock{
				Summary: []string{"safe summary"},
				Detail: llms.ReasoningDetail{
					OpenAI: &llms.OpenAIReasoningDetail{
						Source:           llms.LLMTypeChatGPT,
						EncryptedContent: secret,
					},
				},
			}),
		))

	assert.NotContains(t, msg.ToMarkdown(), secret)
	assert.NotContains(t, msg.LogValue().String(), secret)
	assert.Contains(t, msg.ToMarkdown(), "answer")
}

// Legacy message JSON remains readable after the block migration.
func TestMessageJSONCompatibility(t *testing.T) {
	t.Parallel()

	legacy := `{"id":"id","role":"assistant","content":"hello","tool_calls":[{"id":"call","name":"tool","args":{"n":1},"raw_args":"{\"n\":1}"}]}`

	var msg llms.Message
	require.NoError(t, json.Unmarshal([]byte(legacy), &msg))
	assert.Equal(t, "hello", msg.TextContent())
	require.Len(t, msg.ToolCalls(), 1)

	encoded, err := json.Marshal(&msg)
	require.NoError(t, err)
	assert.Contains(t, string(encoded), `"blocks"`)
	assert.NotContains(t, string(encoded), `"content":`)
	assert.NotContains(t, string(encoded), `"tool_calls":`)
}

func TestToolRoleRequiresExactlyOneCall(t *testing.T) {
	t.Parallel()

	for _, count := range []int{0, 2} {
		blocks := make([]llms.Block, count)
		for i := range blocks {
			blocks[i] = llms.ToolContentBlock(llms.ToolCall{ID: "call", Name: "tool"})
		}

		msg := llms.NewMessage(llms.WithRole(llms.RoleTool), llms.WithBlocks(blocks...))
		require.Error(t, msg.OK())
	}

	msg := llms.NewMessage(
		llms.WithRole(llms.RoleTool),
		llms.WithToolCalls(llms.ToolCall{ID: "call", Name: "tool"}),
	)
	require.NoError(t, msg.OK())
}
