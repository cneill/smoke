package llms

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
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

func (c *Claude) Type() LLMType               { return LLMTypeClaude }
func (c *Claude) ModelName() string           { return string(c.opts.Model) }
func (c *Claude) RequiresSessionSystem() bool { return false }

func (c *Claude) getSessionMessages(session *Session) []anthropic.MessageParam {
	results := make([]anthropic.MessageParam, len(session.Messages))

	for num, msg := range session.Messages {
		switch msg.Role {
		case RoleAssistant:
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
				}
			}

			results[num] = anthropic.NewAssistantMessage(contentBlocks...)
		case RoleSystem:
			// Anthropic defines the system prompt outside of messages
		case RoleUser:
			results[num] = anthropic.NewUserMessage(anthropic.NewTextBlock(msg.Content))
		case RoleTool:
			content := msg.Content
			if content == "" {
				content = "[no output]" // can't be empty?
			}

			results[num] = anthropic.NewUserMessage(anthropic.NewToolResultBlock(msg.ToolCallID, content, msg.Error != nil))
		case RoleUnknown:
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
			properties[param.Key] = map[string]any{
				"type":        param.Type,
				"description": param.Description,
			}

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

func (c *Claude) SendSession(ctx context.Context, session *Session) error {
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

	result, err := c.client.Messages.New(ctx, messageParams)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrCompletion, err)
	}

	if len(result.Content) == 0 {
		return fmt.Errorf("%w: no messages returned", ErrEmptyResponse)
	}

	if result.StopReason == anthropic.StopReasonRefusal {
		return fmt.Errorf("%w: %s", ErrPromptRefused, result.Content[0].Text)
	}

	if err := c.handleToolCalls(ctx, session, result); err != nil {
		return fmt.Errorf("failed to handle tool calls: %w", err)
	}

	return nil
}

func (c *Claude) handleToolCalls(ctx context.Context, session *Session, message *anthropic.Message) error {
	textBuilder := strings.Builder{}
	toolCalls := []anthropic.ToolUseBlock{}
	toolCallNames := []string{}

	for _, block := range message.Content {
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

	msg := NewMessage(
		WithRole(RoleAssistant),
		WithContent(textBuilder.String()),
		WithToolsCalled(toolCallNames...),
		WithToolCallInfo(toolCalls),
	)

	c.logger.Debug("adding assistant message to session", "msg", msg)
	session.AddMessage(msg)

	for _, toolCall := range toolCalls {
		// TODO: refactor to return a *Message from CallTool()?
		name := toolCall.Name

		var (
			content     string
			toolCallErr error
		)

		params, err := c.tools.Tools.Params(name)
		if err != nil {
			return fmt.Errorf("failed to get params for tool %q: %w", name, err)
		}

		args, err := tools.GetArgs([]byte(toolCall.Input), params)
		if err != nil {
			return fmt.Errorf("failed to get args for tool %q: %w", name, err)
		}

		output, err := c.tools.CallTool(name, args)
		if err != nil {
			c.logger.Error("failed to call tool", "tool_name", name, "error", err)
			toolCallErr = fmt.Errorf("failed to call tool %q: %w", name, err)
			content = toolCallErr.Error()
		} else {
			content = output
		}

		toolCallMsg := NewMessage(
			WithRole(RoleTool),
			WithToolCallID(toolCall.ID),
			WithToolCallArgs(args),
			WithContent(content),
			WithError(toolCallErr),
		)

		c.logger.Debug("adding tool result message to session", "msg", toolCallMsg)
		session.AddMessage(toolCallMsg)
	}

	if msg.HasToolCalls() {
		if err := c.SendSession(ctx, session); err != nil {
			return fmt.Errorf("failed to send session with tool call results: %w", err)
		}
	}

	return nil
}
