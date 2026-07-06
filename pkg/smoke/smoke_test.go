package smoke //nolint:testpackage

import (
	"context"
	"sync/atomic"
	"testing"

	"github.com/cneill/smoke/pkg/llmctx/modes"
	"github.com/cneill/smoke/pkg/llms"
	"github.com/cneill/smoke/pkg/tools"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type recordingTool struct {
	calls *atomic.Int64
}

func (r recordingTool) Name() string             { return "recording_tool" }
func (r recordingTool) Description() string      { return "recording tool" }
func (r recordingTool) Examples() tools.Examples { return nil }
func (r recordingTool) Params() tools.Params {
	return tools.Params{
		{
			Key:      "path",
			Type:     tools.ParamTypeString,
			Required: true,
		},
	}
}

func (r recordingTool) Run(context.Context, tools.Args) (*tools.Output, error) {
	r.calls.Add(1)
	return &tools.Output{Text: "executed"}, nil
}

func testSession(t *testing.T, manager *tools.Manager) *llms.Session {
	t.Helper()

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

	return session
}

func TestInvalidArgsReturnFeedback(t *testing.T) {
	t.Parallel()

	t.Run("bad_json", func(t *testing.T) {
		t.Parallel()

		calls := &atomic.Int64{}
		manager, err := tools.NewManager(&tools.ManagerOpts{
			ProjectPath: t.TempDir(),
			SessionName: "test-session",
		})
		require.NoError(t, err)
		manager.SetTools(recordingTool{calls: calls})

		toolCall := llms.ToolCall{
			ID:        "call_123",
			Name:      "recording_tool",
			RawArgs:   `{"path":`,
			ArgsError: "failed to parse arguments for tool call to tool \"recording_tool\": invalid JSON: unexpected EOF",
		}

		msg := toolCallResultMessage(context.Background(), testSession(t, manager), toolCall)

		assert.Equal(t, int64(0), calls.Load(), "invalid arguments should not execute the tool")
		assert.Equal(t, llms.RoleTool, msg.Role)
		require.Len(t, msg.ToolCalls, 1)
		assert.Equal(t, toolCall, msg.ToolCalls[0])
		assert.Contains(t, msg.TextContent, "could not be executed because its arguments were invalid")
		assert.Contains(t, msg.TextContent, toolCall.ArgsError)
		assert.Contains(t, msg.TextContent, toolCall.RawArgs)
		assert.Equal(t, toolCall.ArgsError, msg.Error)
	})

	t.Run("invalid_params", func(t *testing.T) {
		t.Parallel()

		calls := &atomic.Int64{}
		manager, err := tools.NewManager(&tools.ManagerOpts{
			ProjectPath: t.TempDir(),
			SessionName: "test-session",
		})
		require.NoError(t, err)

		rt := recordingTool{calls: calls}
		manager.SetTools(rt)

		rawArgs := `{"not_path": "something"}`
		_, argsErr := manager.GetArgs(rt.Name(), []byte(rawArgs))
		require.Error(t, argsErr)
		require.ErrorContains(t, argsErr, "unknown argument keys: not_path")

		toolCall := llms.ToolCall{
			ID:        "call_123",
			Name:      rt.Name(),
			RawArgs:   rawArgs,
			ArgsError: argsErr.Error(),
		}

		msg := toolCallResultMessage(context.Background(), testSession(t, manager), toolCall)

		assert.Equal(t, int64(0), calls.Load(), "invalid arguments should not execute the tool")
		assert.Equal(t, llms.RoleTool, msg.Role)
		require.Len(t, msg.ToolCalls, 1)
		assert.Equal(t, toolCall, msg.ToolCalls[0])
		assert.Contains(t, msg.TextContent, "could not be executed because its arguments were invalid")
		assert.Contains(t, msg.TextContent, toolCall.ArgsError)
		assert.Contains(t, msg.TextContent, toolCall.RawArgs)
		assert.Equal(t, toolCall.ArgsError, msg.Error)
	})
}

func TestToolCallResultMessageValidArgsExecutesTool(t *testing.T) {
	t.Parallel()

	calls := &atomic.Int64{}
	manager, err := tools.NewManager(&tools.ManagerOpts{
		ProjectPath: t.TempDir(),
		SessionName: "test-session",
	})
	require.NoError(t, err)
	manager.SetTools(recordingTool{calls: calls})

	toolCall := llms.ToolCall{
		ID:   "call_123",
		Name: "recording_tool",
		Args: tools.Args{"path": "README.md"},
	}

	msg := toolCallResultMessage(context.Background(), testSession(t, manager), toolCall)

	assert.Equal(t, int64(1), calls.Load())
	assert.Equal(t, "executed", msg.TextContent)
	assert.Empty(t, msg.Error)
}
