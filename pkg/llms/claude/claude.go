// Package claude contains an implementation of [llms.LLM] for Anthropic's Claude.
package claude

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
	"github.com/anthropics/anthropic-sdk-go/packages/ssestream"
	"github.com/cneill/smoke/pkg/llms"
)

type Claude struct {
	config *llms.Config
	logger *slog.Logger
	client anthropic.Client
}

func configOK(config *llms.Config) error {
	if err := config.OK(); err != nil {
		return fmt.Errorf("base LLM config error: %w", err)
	}

	if config.Temperature < 0 || config.Temperature > 1 {
		return fmt.Errorf("Claude temperature must be between 0 and 1")
	}

	return nil
}

func New(config *llms.Config) (llms.LLM, error) {
	if err := configOK(config); err != nil {
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
	messageParams := c.getMessageNewParams(session)

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
		llms.WithID(response.ID),
		llms.WithRole(llms.RoleAssistant),
		llms.WithContent(textBuilder.String()),
		llms.WithToolsCalled(toolCallNames...),
		llms.WithToolCallInfo(toolCalls),
	)

	return msg, nil
}

func (c *Claude) SendSessionStreaming(ctx context.Context, session *llms.Session, chunkChan chan<- *llms.Message) (*llms.Message, error) {
	defer close(chunkChan)

	messageParams := c.getMessageNewParams(session)

	latest := session.Last()
	if latest != nil {
		c.logger.Debug("sending session", "msg", latest)
	}

	msg := c.newMessage(
		llms.WithRole(llms.RoleAssistant),
		llms.WithIsStreamed(true),
		llms.WithIsInitial(true),
		llms.WithIsChunk(true),
	)

	stream := c.client.Messages.NewStreaming(ctx, messageParams, option.WithMaxRetries(5))
	defer stream.Close()

	accumulator, err := c.handleStreamingResponse(stream, msg, chunkChan)
	if err != nil {
		return nil, fmt.Errorf("failed to handle streaming response: %w", err)
	}

	session.UpdateUsage(accumulator.Usage.InputTokens, accumulator.Usage.OutputTokens)

	c.logger.Debug("token usage", "prompt", accumulator.Usage.InputTokens, "completion", accumulator.Usage.OutputTokens)

	textBuilder := strings.Builder{}
	toolCalls := []anthropic.ToolUseBlock{}
	toolCallNames := []string{}

	for _, block := range accumulator.Content {
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

	msg = msg.Update(
		llms.WithContent(textBuilder.String()),
		llms.WithToolsCalled(toolCallNames...),
		llms.WithToolCallInfo(toolCalls),
		llms.WithIsChunk(false),
		llms.WithIsInitial(false),
		llms.WithIsFinalized(true),
	)

	return msg, nil
}

func (c *Claude) HandleToolCalls(ctx context.Context, msg *llms.Message, session *llms.Session) ([]*llms.Message, error) {
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

		args, err := session.Tools.GetArgs(name, []byte(toolCall.Input))
		if err != nil {
			return nil, fmt.Errorf("failed to get args for tool %q: %w", name, err)
		}

		output, err := session.Tools.CallTool(ctx, name, args)
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

func (c *Claude) getMessageNewParams(session *llms.Session) anthropic.MessageNewParams {
	return anthropic.MessageNewParams{
		Messages:  c.getSessionMessages(session),
		MaxTokens: c.config.MaxTokens,
		Model:     anthropic.Model(c.config.Model),
		System: []anthropic.TextBlockParam{
			{Text: session.SystemMessage},
		},
		Tools:       c.newMessageTools(session),
		Temperature: anthropic.Float(c.config.Temperature),
	}
}

func (c *Claude) newMessageTools(session *llms.Session) []anthropic.ToolUnionParam {
	results := []anthropic.ToolUnionParam{}

	for _, tool := range session.Tools.GetTools() {
		params := tool.Params()

		properties, err := params.JSONSchemaProperties()
		if err != nil {
			c.logger.Error("failed to get JSON Schema properties for tool, skipping", "tool_name", tool.Name(), "error", err)
			continue
		}

		toolDef := anthropic.ToolParam{
			Name:        tool.Name(),
			Description: anthropic.String(tool.Description()),
			InputSchema: anthropic.ToolInputSchemaParam{
				Type:       "object",
				Properties: properties,
				Required:   params.RequiredKeys(),
			},
		}

		results = append(results, anthropic.ToolUnionParam{OfTool: &toolDef})
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

func (c *Claude) handleStreamingResponse(
	stream *ssestream.Stream[anthropic.MessageStreamEventUnion], msg *llms.Message, chunkChan chan<- *llms.Message,
) (anthropic.Message, error) {
	accumulator := anthropic.Message{}

	for stream.Next() {
		chunk := stream.Current()
		if err := accumulator.Accumulate(chunk); err != nil {
			return accumulator, fmt.Errorf("failed to handle message chunk: %w", err)
		}

		chunkType, ok := chunk.AsAny().(anthropic.ContentBlockDeltaEvent)
		if !ok {
			c.logger.Warn("unknown chunk type", "type", fmt.Sprintf("%T", chunk.AsAny()))
			continue
		}

		switch deltaType := chunkType.Delta.AsAny().(type) {
		case anthropic.TextDelta:
			msg = msg.Update(llms.WithChunkContent(deltaType.Text))
			c.logger.Debug("updating chunk with text", "text", deltaType.Text, "current_text", msg.Content)
			// TODO: other delta types?
		default:
			continue
		}

		chunkChan <- msg

		msg = msg.Update(llms.WithIsInitial(false))
	}

	if err := stream.Err(); err != nil {
		return accumulator, fmt.Errorf("%w: streaming: %w", llms.ErrCompletion, err)
	}

	if len(accumulator.Content) == 0 {
		return accumulator, fmt.Errorf("%w: no messages returned", llms.ErrEmptyResponse)
	}

	if accumulator.StopReason == anthropic.StopReasonRefusal {
		return accumulator, fmt.Errorf("%w: %s", llms.ErrPromptRefused, accumulator.Content[0].Text)
	}

	return accumulator, nil
}
