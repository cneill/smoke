// Package claude contains an implementation of [llms.LLM] for Anthropic's Claude.
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

type Claude struct {
	config *llms.Config
	logger *slog.Logger
	client anthropic.Client
}

func New(config *llms.Config) (llms.LLM, error) {
	if err := config.OK(); err != nil {
		return nil, fmt.Errorf("error with Claude options: %w", err)
	}

	client := anthropic.NewClient(
		option.WithAPIKey(config.APIKey),
	)

	claude := &Claude{
		config: config,
		logger: slog.Default().WithGroup(llms.LLMTypeClaude),
		client: client,
	}

	return claude, nil
}

func (c *Claude) LLMInfo() *llms.LLMInfo {
	return &llms.LLMInfo{
		Type:      llms.LLMTypeClaude,
		ModelName: c.config.Model,
	}
}

func (c *Claude) RequiresSessionSystem() bool { return false }

func (c *Claude) SendSession(ctx context.Context, session *llms.Session) (*llms.Message, error) {
	messageParams := anthropic.MessageNewParams{
		Messages:  c.getSessionMessages(session),
		MaxTokens: c.config.MaxTokens,
		Model:     anthropic.Model(c.config.Model),
		System: []anthropic.TextBlockParam{
			{Text: session.SystemMessage},
		},
		Tools: c.newMessageTools(session),
	}

	latest := session.Last()
	if latest != nil {
		c.logger.Debug("sending session", "msg", latest)
	}

	response, err := c.client.Messages.New(ctx, messageParams)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", llms.ErrCompletion, err)
	}

	session.UpdateUsage(response.Usage.InputTokens, response.Usage.OutputTokens)

	c.logger.Debug("token usage", "input", response.Usage.InputTokens, "output", response.Usage.OutputTokens)

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

func (c *Claude) HandleToolCalls(msg *llms.Message, session *llms.Session) ([]*llms.Message, error) {
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

		params, err := session.Tools.Params(name)
		if err != nil {
			return nil, fmt.Errorf("failed to get params for tool %q: %w", name, err)
		}

		args, err := tools.GetArgs([]byte(toolCall.Input), params)
		if err != nil {
			return nil, fmt.Errorf("failed to get args for tool %q: %w", name, err)
		}

		output, err := session.Tools.CallTool(context.TODO(), name, args)
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
			llms.WithToolsCalled(toolCall.Name),
			llms.WithContent(content),
			llms.WithError(toolCallErr),
		)

		results[toolCallName] = toolCallResultMsg
	}

	return results, nil
}

func (c *Claude) newMessageTools(session *llms.Session) []anthropic.ToolUnionParam {
	results := make([]anthropic.ToolUnionParam, len(session.Tools.Tools))

	for toolNum, tool := range session.Tools.Tools {
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
			results[num] = c.getAssistantMessage(msg)
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

func (c *Claude) getAssistantMessage(msg *llms.Message) anthropic.MessageParam {
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

	return anthropic.NewAssistantMessage(contentBlocks...)
}
