package claude //nolint:testpackage // Tests exercise provider conversion internals.

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/cneill/smoke/pkg/llmctx/modes"
	"github.com/cneill/smoke/pkg/llms"
	"github.com/cneill/smoke/pkg/providers/base"
	"github.com/cneill/smoke/pkg/tools"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type testTool struct{}

func (testTool) Name() string             { return "test_tool" }
func (testTool) Description() string      { return "test tool" }
func (testTool) Examples() tools.Examples { return nil }
func (testTool) Run(context.Context, tools.Args) (*tools.Output, error) {
	return &tools.Output{Text: "ok"}, nil
}

func (testTool) Params() tools.Params {
	return tools.Params{{Key: "path", Type: tools.ParamTypeString, Required: true}}
}

func TestHandleResponsePreservesOrderedBlocksAndReasoning(t *testing.T) {
	t.Parallel()

	conv := testConversation(t)
	providerBlocks := contentBlocks(t, `[
		{"type":"thinking","thinking":"visible thinking","signature":"signature"},
		{"type":"text","text":"first"},
		{"type":"tool_use","id":"call","name":"test_tool","input":{"path":"README.md"}},
		{"type":"redacted_thinking","data":"redacted-data"},
		{"type":"text","text":"second"}
	]`)

	event := handleResponseEvent(t, conv, "message", providerBlocks)
	requested, ok := event.(llms.EventToolCallsRequested)
	require.True(t, ok)

	msg := requested.Message
	require.Len(t, msg.Blocks, 5)
	assert.Equal(t, []llms.BlockType{
		llms.BlockTypeReasoning,
		llms.BlockTypeText,
		llms.BlockTypeToolCall,
		llms.BlockTypeReasoning,
		llms.BlockTypeText,
	}, blockTypes(msg.Blocks))
	assert.Equal(t, "signature", msg.Blocks[0].Reasoning.Detail.Anthropic.Signature)
	assert.Equal(t, "redacted-data", msg.Blocks[3].Reasoning.Detail.Anthropic.Data)
	assert.True(t, msg.Blocks[3].Reasoning.Detail.Anthropic.Redacted)
	assert.JSONEq(t, `{"path":"README.md"}`, msg.Blocks[2].ToolCall.ToolCall.RawArgs)
}

func TestGetSessionMessagesReplaysCompatibleReasoningInOrder(t *testing.T) {
	t.Parallel()

	conv := testConversation(t)
	conv.Session().Messages = []*llms.Message{llms.NewMessage(
		llms.WithRole(llms.RoleAssistant),
		llms.WithBlocks(
			anthropicReasoningBlock("thinking", "signature"),
			llms.TextContentBlock("answer"),
			openAIReasoningBlock(),
			llms.ToolContentBlock(llms.ToolCall{ID: "call", Name: "test_tool", RawArgs: `{"path":"README.md"}`}),
			llms.ReasoningContentBlock(llms.ReasoningBlock{
				Detail: llms.ReasoningDetail{
					Anthropic: &llms.AnthropicReasoningDetail{
						Source:   llms.LLMTypeClaude,
						Redacted: true,
						Data:     "redacted",
					},
				},
			}),
		),
	)}

	messages := conv.getSessionMessages(conv.Session())
	require.Len(t, messages, 1)
	require.Len(t, messages[0].Content, 4)
	assert.NotNil(t, messages[0].Content[0].OfThinking)
	assert.Equal(t, "signature", messages[0].Content[0].OfThinking.Signature)
	assert.NotNil(t, messages[0].Content[1].OfText)
	assert.NotNil(t, messages[0].Content[2].OfToolUse)
	assert.NotNil(t, messages[0].Content[3].OfRedactedThinking)
	assert.Equal(t, "redacted", messages[0].Content[3].OfRedactedThinking.Data)
}

func TestProviderToolCallInvalidArgsIsRecordedWithoutResponseError(t *testing.T) {
	t.Parallel()

	conv := testConversation(t)
	blocks := contentBlocks(t, `[{"type":"tool_use","id":"call","name":"test_tool","input":{"other":true}}]`)
	event := handleResponseEvent(t, conv, "message", blocks)
	requested, ok := event.(llms.EventToolCallsRequested)
	require.True(t, ok)

	calls := requested.Message.ToolCalls()
	require.Len(t, calls, 1)
	assert.Equal(t, `{"other":true}`, calls[0].RawArgs)
	assert.NotEmpty(t, calls[0].ArgsError)
}

func testConversation(t *testing.T) *conversation {
	t.Helper()
	manager, err := tools.NewManager(&tools.ManagerOpts{ProjectPath: t.TempDir(), SessionName: "test"})
	require.NoError(t, err)
	manager.SetTools(testTool{})

	session, err := llms.NewSession(&llms.SessionOpts{
		Name:          "test",
		SystemMessage: "system",
		Tools:         manager,
		Mode:          modes.ModeWork,
		Config:        &llms.Config{ModelInfo: llms.ModelInfo{Provider: llms.LLMTypeClaude, Model: "model"}},
	},
	)
	require.NoError(t, err)

	send := func(context.Context) error { return nil }
	baseConv, _, err := base.NewConversation(context.Background(), &base.ConversationOpts{
		Session: session,
		LLMInfo: &llms.LLMInfo{
			Type:      llms.LLMTypeClaude,
			ModelName: "model",
		},
		Config:       session.Config,
		SendStream:   send,
		SendNoStream: send,
	})
	require.NoError(t, err)

	return &conversation{Conversation: baseConv}
}

func handleResponseEvent(
	t *testing.T,
	conv *conversation,
	id string,
	blocks []anthropic.ContentBlockUnion,
) llms.Event {
	t.Helper()

	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
	defer cancel()

	errChan := make(chan error, 1)
	go func() {
		errChan <- conv.handleResponse(ctx, id, blocks)
	}()

	var event llms.Event
	select {
	case event = <-conv.Events():
	case <-ctx.Done():
		require.FailNow(t, "timed out waiting for conversation event")
	}

	require.NoError(t, <-errChan)

	return event
}

func contentBlocks(t *testing.T, raw string) []anthropic.ContentBlockUnion {
	t.Helper()

	var blocks []anthropic.ContentBlockUnion

	require.NoError(t, json.Unmarshal([]byte(raw), &blocks))

	return blocks
}

func blockTypes(blocks []llms.Block) []llms.BlockType {
	result := make([]llms.BlockType, len(blocks))
	for i := range blocks {
		result[i] = blocks[i].Type()
	}

	return result
}

func anthropicReasoningBlock(thinking, signature string) llms.Block {
	return llms.ReasoningContentBlock(llms.ReasoningBlock{
		Summary: []string{thinking},
		Detail: llms.ReasoningDetail{
			Anthropic: &llms.AnthropicReasoningDetail{
				Source:    llms.LLMTypeClaude,
				Thinking:  thinking,
				Signature: signature,
			},
		},
	})
}

func openAIReasoningBlock() llms.Block {
	return llms.ReasoningContentBlock(llms.ReasoningBlock{
		Detail: llms.ReasoningDetail{
			OpenAI: &llms.OpenAIReasoningDetail{
				Source: llms.LLMTypeChatGPT,
				ID:     "reason",
			},
		},
	})
}
