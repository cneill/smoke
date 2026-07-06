package chatgpt //nolint:testpackage

import (
	"context"
	"testing"

	"github.com/cneill/smoke/pkg/llmctx/modes"
	"github.com/cneill/smoke/pkg/llms"
	"github.com/cneill/smoke/pkg/providers/base"
	"github.com/cneill/smoke/pkg/tools"
	"github.com/openai/openai-go/v3"
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
			Provider: llms.LLMTypeChatGPT,
			Model:    "test-model",
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
			Provider: llms.LLMTypeChatGPT,
			Model:    "test-model",
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
