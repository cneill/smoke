package chatgpt //nolint:testpackage

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/cneill/smoke/pkg/llmctx/modes"
	"github.com/cneill/smoke/pkg/llms"
	"github.com/cneill/smoke/pkg/providers/base"
	"github.com/cneill/smoke/pkg/tools"
	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/responses"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type testTool struct{}

func (t testTool) Name() string             { return "test_tool" }
func (t testTool) Description() string      { return "test tool" }
func (t testTool) Examples() tools.Examples { return nil }
func (t testTool) Run(context.Context, tools.Args) (*tools.Output, error) {
	return &tools.Output{Text: "ok"}, nil
}

func (t testTool) Params() tools.Params {
	return tools.Params{
		{
			Key:      "path",
			Type:     tools.ParamTypeString,
			Required: true,
		},
	}
}

func testConversation(t *testing.T) *conversation {
	t.Helper()

	manager, err := tools.NewManager(&tools.ManagerOpts{
		ProjectPath: t.TempDir(),
		SessionName: "test-session",
	})
	require.NoError(t, err)
	manager.SetTools(testTool{})

	session, err := llms.NewSession(&llms.SessionOpts{
		Name:          "test-session",
		SystemMessage: "system",
		Tools:         manager,
		Mode:          modes.ModeWork,
		Config: &llms.Config{
			ModelInfo: llms.ModelInfo{Provider: llms.LLMTypeChatGPT, Model: "test-model"},
		},
	})
	require.NoError(t, err)
	conv, _, err := NewConversation(context.Background(), openai.Client{}, &base.ConversationOpts{
		Session: session,
		LLMInfo: &llms.LLMInfo{
			Type:      llms.LLMTypeChatGPT,
			ModelName: "test-model",
		},
		Config: &llms.Config{
			ModelInfo: llms.ModelInfo{Provider: llms.LLMTypeChatGPT, Model: "test-model"},
		},
		Stream: false,
	})
	require.NoError(t, err)

	chatGPTConv, ok := conv.(*conversation)
	require.True(t, ok)

	return chatGPTConv
}

func TestNewToolCallPreservesRawArgsAndRecordsParseErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		toolName    string
		rawArgs     string
		errorIs     error
		expectsArgs bool
	}{
		{
			name:     "malformed_json",
			toolName: "test_tool",
			rawArgs:  `{"path":`,
			errorIs:  tools.ErrInvalidJSON,
		},
		{
			name:     "missing_required_key",
			toolName: "test_tool",
			rawArgs:  `{}`,
			errorIs:  tools.ErrMissingKeys,
		},
		{
			name:     "unknown_tool",
			toolName: "unknown_tool",
			rawArgs:  `{"path":"README.md"}`,
			errorIs:  tools.ErrUnknownTool,
		},
		{
			name:        "valid_args",
			toolName:    "test_tool",
			rawArgs:     `{"path":"README.md"}`,
			expectsArgs: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			conv := testConversation(t)
			toolCall := conv.newToolCall("call_123", test.toolName, test.rawArgs)

			assert.Equal(t, "call_123", toolCall.ID)
			assert.Equal(t, test.toolName, toolCall.Name)
			assert.Equal(t, test.rawArgs, toolCall.RawArgs)

			if test.errorIs != nil {
				require.True(t, toolCall.InvalidArgs())
				require.Error(t, toolCall.GetArgsErr())
				assert.Contains(t, toolCall.ArgsError, test.errorIs.Error())
			} else {
				assert.False(t, toolCall.InvalidArgs())
			}

			if test.expectsArgs {
				require.NotNil(t, toolCall.Args)
				assert.Equal(t, "README.md", *toolCall.Args.GetString("path"))
			}
		})
	}
}

func TestNewToolCallDoesNotReturnConversationErrorForInvalidArgs(t *testing.T) {
	t.Parallel()

	conv := testConversation(t)
	toolCall := conv.newToolCall("call_123", "test_tool", `{"path":`)

	require.True(t, toolCall.InvalidArgs())
	assert.Contains(t, toolCall.ArgsError, tools.ErrInvalidJSON.Error())
}

func TestOutputToMessagePreservesOrderedBlocksAndFirstMessageID(t *testing.T) {
	t.Parallel()

	conv := testConversation(t)
	output := responseOutput(t, `[
		{
			"type":"reasoning","id":"reason_1",
			"summary":[{"type":"summary_text","text":"summary"}],
			"content":[{"type":"reasoning_text","text":"reasoning"}],
			"encrypted_content":"encrypted","status":"completed"
		},
		{
			"type":"message","id":"message_1","role":"assistant","status":"completed",
			"phase":"commentary","content":[{"type":"output_text","text":"first","annotations":[]}]
		},
		{"type":"function_call","id":"item_1","call_id":"call_1","name":"test_tool","arguments":"{\"path\":","status":"completed"},
		{
			"type":"message","id":"message_2","role":"assistant","status":"completed",
			"phase":"final_answer","content":[{"type":"output_text","text":"second","annotations":[]}]
		}
	]`)

	msg, err := conv.outputToMessage(output)
	require.NoError(t, err)
	require.Len(t, msg.Blocks, 4)
	assert.Equal(t, "message_1", msg.ID)
	assert.Equal(t, []llms.BlockType{
		llms.BlockTypeReasoning,
		llms.BlockTypeText,
		llms.BlockTypeToolCall,
		llms.BlockTypeText,
	}, blockTypes(msg.Blocks))
	assert.Equal(t, "commentary", msg.Blocks[1].Text.Phase)
	assert.Equal(t, "final_answer", msg.Blocks[3].Text.Phase)
	assert.Equal(t, "encrypted", msg.Blocks[0].Reasoning.Detail.OpenAI.EncryptedContent)
	assert.Equal(t, llms.LLMTypeChatGPT, string(msg.Blocks[0].Reasoning.Detail.OpenAI.Source))
	assert.Equal(t, `{"path":`, msg.Blocks[2].ToolCall.ToolCall.RawArgs)
	assert.NotEmpty(t, msg.Blocks[2].ToolCall.ToolCall.ArgsError)
}

func TestGetInputFromSessionReplaysOnlyMatchingReasoningOrigin(t *testing.T) {
	t.Parallel()

	conv := testConversation(t)
	conv.Session().Messages = []*llms.Message{llms.NewMessage(
		llms.WithRole(llms.RoleAssistant),
		llms.WithBlocks(
			llms.PhasedTextContentBlock("commentary", "commentary"),
			openAIReasoningBlock(llms.LLMTypeChatGPT, "chatgpt-encrypted"),
			openAIReasoningBlock(llms.LLMTypeGrok, "grok-encrypted"),
			llms.ToolContentBlock(llms.ToolCall{ID: "call", Name: "test_tool", RawArgs: `{"path":"README.md"}`}),
		),
	)}

	input := conv.getInputFromSession(conv.Session()).OfInputItemList
	require.Len(t, input, 3)
	assert.NotNil(t, input[0].OfMessage)
	assert.Equal(t, responses.EasyInputMessagePhaseCommentary, input[0].OfMessage.Phase)
	assert.NotNil(t, input[1].OfReasoning)
	assert.Equal(t, "chatgpt-encrypted", input[1].OfReasoning.EncryptedContent.Value)
	assert.NotNil(t, input[2].OfFunctionCall)
	assert.JSONEq(t, `{"path":"README.md"}`, input[2].OfFunctionCall.Arguments)
}

func TestEncryptedReasoningIncludeCapabilityGate(t *testing.T) {
	t.Parallel()

	conv := testConversation(t)
	params := conv.getNewResponsesParams()
	assert.Contains(t, params.Include, responses.ResponseIncludableReasoningEncryptedContent)

	conv.Config().ModelInfo.Provider = llms.LLMTypeOllama
	params = conv.getNewResponsesParams()
	assert.Empty(t, params.Include)
}

func responseOutput(t *testing.T, raw string) []responses.ResponseOutputItemUnion {
	t.Helper()

	var output []responses.ResponseOutputItemUnion

	require.NoError(t, json.Unmarshal([]byte(raw), &output))

	return output
}

func blockTypes(blocks []llms.Block) []llms.BlockType {
	result := make([]llms.BlockType, len(blocks))
	for i := range blocks {
		result[i] = blocks[i].Type()
	}

	return result
}

func openAIReasoningBlock(source llms.LLMType, encrypted string) llms.Block {
	return llms.ReasoningContentBlock(llms.ReasoningBlock{
		Summary: []string{"summary"},
		Detail: llms.ReasoningDetail{
			OpenAI: &llms.OpenAIReasoningDetail{Source: source, ID: "reason", EncryptedContent: encrypted, Status: "completed"},
		},
	})
}
