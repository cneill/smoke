package claude

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
	"github.com/cneill/smoke/pkg/llms"
	"github.com/cneill/smoke/pkg/tools"
)

type ClaudeOpts struct {
	APIKey       string
	Model        anthropic.Model
	MaxTokens    int64
	ToolsManager *tools.Manager
}

func (c *ClaudeOpts) OK() error {
	switch {
	case c.APIKey == "":
		return fmt.Errorf("missing api key")
	case c.Model == "":
		return fmt.Errorf("missing model")
	case c.MaxTokens <= 0:
		return fmt.Errorf("max tokens must be >0")
	case c.ToolsManager == nil:
		return fmt.Errorf("must supply a tools manager instance")
	}

	return nil
}

type Claude struct {
	opts   *ClaudeOpts
	logger *slog.Logger
	tools  *tools.Manager
	client anthropic.Client
}

func NewClaude(opts *ClaudeOpts) (*Claude, error) {
	if err := opts.OK(); err != nil {
		return nil, fmt.Errorf("error with Claude options: %w", err)
	}

	client := anthropic.NewClient(
		option.WithAPIKey(opts.APIKey),
	)

	claude := &Claude{
		opts:   opts,
		logger: slog.Default().WithGroup("claude"),
		tools:  opts.ToolsManager,
		client: client,
	}

	return claude, nil
}

func (c *Claude) LLMInfo() *llms.LLMInfo {
	return &llms.LLMInfo{
		Type:      llms.LLMTypeClaude,
		ModelName: string(c.opts.Model),
	}
}

func (c *Claude) RequiresSessionSystem() bool { return false }

func (c *Claude) newMessage(opts ...llms.MessageOpt) *llms.Message {
	msg := llms.NewMessage(
		llms.WithLLMInfo(c.LLMInfo()),
	)

	for _, opt := range opts {
		msg = opt(msg)
	}

	return msg
}

func (c *Claude) getSessionMessages(session *llms.Session) []anthropic.MessageParam {
	results := make([]anthropic.MessageParam, len(session.Messages))

	for num, msg := range session.Messages {
		switch msg.Role {
		case llms.RoleAssistant:
			contentBlocks := []anthropic.ContentBlockParamUnion{}

			if strings.TrimSpace(msg.Content) != "" {
				contentBlocks = append(contentBlocks, anthropic.NewTextBlock(msg.Content))
			}

			if msg.HasToolCalls() {
				rawCalls, ok := msg.ToolCallInfo.([]anthropic.ToolUseBlock)
				if ok {
					for _, toolCall := range rawCalls {
						toolUseParam := toolCall.ToParam()
						toolUseContentBlock := anthropic.ContentBlockParamUnion{
							OfToolUse: &toolUseParam,
						}
						contentBlocks = append(contentBlocks, toolUseContentBlock)
					}
				} else {
					c.logger.Warn("got ToolCallInfo of unexpected type", "type", fmt.Sprintf("%T", msg.ToolCallInfo))
				}
			}

			results[num] = anthropic.NewAssistantMessage(contentBlocks...)
		case llms.RoleSystem:
			// Anthropic defines the system prompt outside of messages
		case llms.RoleUser:
			results[num] = anthropic.NewUserMessage(anthropic.NewTextBlock(msg.Content))
		case llms.RoleTool:
			content := msg.Content
			if content == "" {
				content = "[no output]" // can't be empty?
			}

			results[num] = anthropic.NewUserMessage(anthropic.NewToolResultBlock(msg.ToolCallID, content, msg.Error != nil))
		case llms.RoleUnknown:
			c.logger.Warn("got message with unknown role", "message", msg.Content)
		}
	}

	return results
}

func (c *Claude) NewMessageTools() []anthropic.ToolUnionParam {
	results := make([]anthropic.ToolUnionParam, len(c.tools.Tools))

	for toolNum, tool := range c.tools.Tools {
		properties := map[string]any{}
		requiredKeys := []string{}

		for _, param := range tool.Params() {
			keyParams := map[string]any{
				"type":        param.Type,
				"description": param.Description,
			}

			if param.Type == tools.ParamTypeArray {
				keyParams["items"] = map[string]any{
					"type": param.ItemType,
				}
			}

			properties[param.Key] = keyParams

			if param.Required {
				requiredKeys = append(requiredKeys, param.Key)
			}
		}

		schema := anthropic.ToolInputSchemaParam{
			Properties: properties,
			Required:   requiredKeys,
			Type:       "object",
		}

		toolParam := anthropic.ToolParam{
			Name:        tool.Name(),
			Description: anthropic.String(tool.Description()),
			InputSchema: schema,
		}

		results[toolNum] = anthropic.ToolUnionParam{OfTool: &toolParam}
	}

	return results
}

func (c *Claude) SendSession(ctx context.Context, session *llms.Session) (*llms.Message, error) {
	messageParams := anthropic.MessageNewParams{
		Messages:  c.getSessionMessages(session),
		MaxTokens: c.opts.MaxTokens,
		Model:     c.opts.Model,
		System: []anthropic.TextBlockParam{
			{Text: session.SystemMessage},
		},
		Tools: c.NewMessageTools(),
	}

	latest := session.Last()
	if latest != nil {
		c.logger.Debug("sending session", "msg", latest)
	}

	response, err := c.client.Messages.New(ctx, messageParams)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", llms.ErrCompletion, err)
	}

	if len(response.Content) == 0 {
		return nil, fmt.Errorf("%w: no messages returned", llms.ErrEmptyResponse)
	}

	if response.StopReason == anthropic.StopReasonRefusal {
		return nil, fmt.Errorf("%w: %s", llms.ErrPromptRefused, response.Content[0].Text)
	}

	textBuilder := strings.Builder{}
	toolCalls := []anthropic.ToolUseBlock{}
	toolCallNames := []string{}

	for _, block := range response.Content {
		switch block := block.AsAny().(type) {
		case anthropic.TextBlock:
			// TODO: citations?
			if strings.TrimSpace(block.Text) != "" {
				textBuilder.WriteString(block.Text + "\n")
			}
		case anthropic.ToolUseBlock:
			toolCalls = append(toolCalls, block)
			toolCallNames = append(toolCallNames, block.Name)
		}
	}

	msg := c.newMessage(
		llms.WithRole(llms.RoleAssistant),
		llms.WithContent(textBuilder.String()),
		llms.WithToolsCalled(toolCallNames...),
		llms.WithToolCallInfo(toolCalls),
	)

	return msg, nil
}

func (c *Claude) HandleToolCalls(msg *llms.Message) ([]*llms.Message, error) {
	if !msg.HasToolCalls() {
		return nil, llms.ErrNoToolCalls
	}

	toolCalls, ok := msg.ToolCallInfo.([]anthropic.ToolUseBlock)
	if !ok {
		return nil, fmt.Errorf("tool call info was of unexpected type: %T", msg.ToolCallInfo)
	}

	results := make([]*llms.Message, len(toolCalls))

	for toolCallName, toolCall := range toolCalls {
		name := toolCall.Name

		var (
			content     string
			toolCallErr error
		)

		params, err := c.tools.Tools.Params(name)
		if err != nil {
			return nil, fmt.Errorf("failed to get params for tool %q: %w", name, err)
		}

		args, err := tools.GetArgs([]byte(toolCall.Input), params)
		if err != nil {
			return nil, fmt.Errorf("failed to get args for tool %q: %w", name, err)
		}

		output, err := c.tools.CallTool(name, args)
		if err != nil {
			c.logger.Error("failed to call tool", "tool_name", name, "error", err)
			toolCallErr = fmt.Errorf("failed to call tool %q: %w", name, err)
			content = toolCallErr.Error()
		} else {
			content = output
		}

		toolCallResultMsg := c.newMessage(
			llms.WithRole(llms.RoleTool),
			llms.WithToolCallID(toolCall.ID),
			llms.WithToolCallArgs(args),
			llms.WithContent(content),
			llms.WithError(toolCallErr),
		)

		results[toolCallName] = toolCallResultMsg
	}

	return results, nil
}
