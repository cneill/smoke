// Package chatgpt contains an implementation of [llms.LLM] for OpenAI's ChatGPT.
package chatgpt

import (
	"context"
	"fmt"
	"strings"

	"github.com/cneill/smoke/pkg/llms"
	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
)

type ChatGPT struct {
	Config *llms.Config
	Client openai.Client
}

func configOK(config *llms.Config) error {
	if err := config.OK(); err != nil {
		return fmt.Errorf("base LLM config error: %w", err)
	}

	if config.Temperature < 0 || config.Temperature > 2 {
		return fmt.Errorf("ChatGPT temperature must be between 0 and 2")
	}

	return nil
}

func New(config *llms.Config) (llms.LLM, error) {
	if err := configOK(config); err != nil {
		return nil, fmt.Errorf("error with ChatGPT options: %w", err)
	}

	client := openai.NewClient(
		option.WithAPIKey(config.APIKey),
	)

	chatGPT := &ChatGPT{
		Config: config,
		Client: client,
	}

	return chatGPT, nil
}

func (c *ChatGPT) LLMInfo() *llms.LLMInfo {
	return &llms.LLMInfo{
		Type:      llms.LLMTypeChatGPT,
		ModelName: c.Config.Model,
	}
}
func (c *ChatGPT) RequiresSessionSystem() bool { return true }

func (c *ChatGPT) StartConversation(ctx context.Context, session *llms.Session) llms.Conversation {
	newCtx, cancel := context.WithCancelCause(ctx)

	conv := &conversation{
		id:           session.Name,
		stream:       c.shouldStream(),
		cancel:       cancel,
		eventChan:    make(chan llms.Event),
		continueChan: make(chan struct{}),
		session:      session, // TODO: read-only view
		llmInfo:      c.LLMInfo(),
		client:       c.Client,
		config:       c.Config,
	}

	go conv.run(newCtx)

	return conv
}

func (c *ChatGPT) shouldStream() bool {
	// GPT-5 requires photo ID verification for streaming...
	return !strings.Contains(c.Config.Model, "gpt-5")
}

// func (c *ChatGPT) SendSession(ctx context.Context, session *llms.Session) (*llms.Message, error) {
// 	options := c.getNewCompletionParams(session)
//
// 	latest := session.Last()
// 	if latest != nil {
// 		c.logger.Debug("sending session", "message", latest)
// 	}
//
// 	result, err := c.client.Chat.Completions.New(ctx, options, option.WithMaxRetries(5))
// 	if err != nil {
// 		return nil, fmt.Errorf("%w: %w", llms.ErrCompletion, err)
// 	}
//
// 	session.UpdateUsage(result.Usage.PromptTokens, result.Usage.CompletionTokens)
//
// 	c.logger.Debug("token usage", "prompt", result.Usage.PromptTokens, "completion", result.Usage.CompletionTokens)
//
// 	if len(result.Choices) == 0 {
// 		return nil, fmt.Errorf("%w: no messages returned", llms.ErrEmptyResponse)
// 	}
//
// 	if refusal := result.Choices[0].Message.Refusal; refusal != "" {
// 		return nil, fmt.Errorf("%w: %s", llms.ErrPromptRefused, refusal)
// 	}
//
// 	response := result.Choices[0].Message
//
// 	msg := c.newMessage(
// 		llms.WithID(result.ID),
// 		llms.WithRole(llms.RoleAssistant),
// 		llms.WithContent(response.Content),
// 		llms.WithToolsCalled(c.getToolCallNames(response.ToolCalls)...),
// 		// We convert this ToParam() because it's the only way to get streaming responses below to work...
// 		llms.WithToolCallInfo(response.ToParam().OfAssistant.ToolCalls),
// 	)
//
// 	return msg, nil
// }

// func (c *ChatGPT) SendSessionStreaming(ctx context.Context, session *llms.Session, chunkChan chan<- *llms.Message) (*llms.Message, error) {
// 	defer close(chunkChan)
//
// 	options := c.getNewCompletionParams(session)
//
// 	latest := session.Last()
// 	if latest != nil {
// 		c.logger.Debug("sending session streaming", "message", latest)
// 	}
//
// 	msg := c.newMessage(
// 		llms.WithRole(llms.RoleAssistant),
// 		// llms.WithIsStreamed(true),
// 		// llms.WithIsInitial(true),
// 		// llms.WithIsChunk(true),
// 	)
//
// 	stream := c.client.Chat.Completions.NewStreaming(ctx, options, option.WithMaxRetries(5))
// 	defer stream.Close()
//
// 	accumulator, err := c.handleStreamingResponse(stream, msg, chunkChan)
// 	if err != nil {
// 		return nil, fmt.Errorf("error handling streaming response: %w", err)
// 	}
//
// 	c.logger.Debug("token usage", "prompt", accumulator.Usage.PromptTokens, "completion", accumulator.Usage.CompletionTokens)
//
// 	if len(accumulator.Choices) == 0 {
// 		return nil, fmt.Errorf("%w: no messages returned", llms.ErrEmptyResponse)
// 	}
//
// 	response := accumulator.Choices[0].Message
//
// 	session.UpdateUsage(accumulator.Usage.PromptTokens, accumulator.Usage.CompletionTokens)
//
// 	msg = msg.Update(
// 		llms.WithContent(response.Content),
// 		llms.WithToolsCalled(c.getToolCallNames(response.ToolCalls)...),
// 		llms.WithToolCallInfo(response.ToParam().OfAssistant.ToolCalls),
// 		// llms.WithIsChunk(false),
// 		// llms.WithIsInitial(false),
// 		// llms.WithIsFinalized(true),
// 	)
//
// 	slog.Debug("FINAL MESSAGE", "message", msg)
//
// 	return msg, nil
// }

// func (c *ChatGPT) HandleToolCalls(ctx context.Context, msg *llms.Message, session *llms.Session) ([]*llms.Message, error) {
// 	if !msg.HasToolCalls() {
// 		return nil, llms.ErrNoToolCalls
// 	}
//
// 	toolCalls, ok := msg.ToolCallInfo.([]openai.ChatCompletionMessageToolCallUnionParam)
// 	if !ok {
// 		return nil, fmt.Errorf(
// 			"got unexpected type for ToolCallInfo (expecting []openai.ChatCompletionMessageToolCallUnionParam): %T",
// 			msg.ToolCallInfo)
// 	}
//
// 	results := make([]*llms.Message, len(toolCalls))
//
// 	for toolCallNum, toolCall := range toolCalls {
// 		name := toolCall.OfFunction.Function.Name
//
// 		var (
// 			content     string
// 			toolCallErr error
// 		)
//
// 		args, err := session.Tools.GetArgs(name, []byte(toolCall.OfFunction.Function.Arguments))
// 		if err != nil {
// 			return nil, fmt.Errorf("failed to get args for tool %q: %w", name, err)
// 		}
//
// 		output, err := session.Tools.CallTool(ctx, name, args)
// 		if err != nil {
// 			c.logger.Error("failed to call tool", "tool_name", name, "error", err)
// 			toolCallErr = fmt.Errorf("failed to call tool %q: %w", name, err)
// 			content = toolCallErr.Error()
// 		} else {
// 			content = output
// 		}
//
// 		toolCallResultMsg := c.newMessage(
// 			llms.WithRole(llms.RoleTool),
// 			// llms.WithIsChunk(false),
// 			// llms.WithIsStreamed(false),
// 			llms.WithToolCallID(toolCall.OfFunction.ID),
// 			llms.WithToolCallArgs(args),
// 			llms.WithToolsCalled(toolCall.OfFunction.Function.Name),
// 			llms.WithContent(content),
// 			llms.WithError(err),
// 		)
//
// 		results[toolCallNum] = toolCallResultMsg
// 	}
//
// 	c.logger.Debug("returning tool call results", "results", results, "num", len(results))
//
// 	return results, nil
// }

// func (c *ChatGPT) newMessage(opts ...llms.MessageOpt) *llms.Message {
// 	msg := llms.NewMessage(
// 		llms.WithLLMInfo(c.LLMInfo()),
// 	)
//
// 	for _, opt := range opts {
// 		msg = opt(msg)
// 	}
//
// 	return msg
// }

// func (c *ChatGPT) getNewCompletionParams(session *llms.Session) openai.ChatCompletionNewParams {
// 	return openai.ChatCompletionNewParams{
// 		MaxCompletionTokens: openai.Int(c.config.MaxTokens),
// 		Messages:            c.getSessionMessages(session),
// 		Model:               c.config.Model,
// 		N:                   openai.Int(1),
// 		Tools:               c.completionTools(session),
// 		Temperature:         openai.Float(c.config.Temperature),
// 	}
// }

// func (c *ChatGPT) completionTools(session *llms.Session) []openai.ChatCompletionToolUnionParam {
// 	results := []openai.ChatCompletionToolUnionParam{}
//
// 	for _, tool := range session.Tools.GetTools() {
// 		params := tool.Params()
//
// 		properties, err := params.JSONSchemaProperties()
// 		if err != nil {
// 			c.logger.Error("failed to get JSON Schema properties for tool, skipping", "tool_name", tool.Name(), "error", err)
// 			continue
// 		}
//
// 		toolDef := openai.ChatCompletionFunctionTool(openai.FunctionDefinitionParam{
// 			Name:        tool.Name(),
// 			Description: openai.String(tool.Description()),
// 			Parameters: openai.FunctionParameters{
// 				"type":       tools.ParamTypeObject,
// 				"properties": properties,
// 				"required":   params.RequiredKeys(),
// 			},
// 		})
// 		results = append(results, toolDef)
// 	}
//
// 	return results
// }

// func (c *ChatGPT) getToolCallNames(toolCalls []openai.ChatCompletionMessageToolCallUnion) []string {
// 	results := []string{}
// 	for _, toolCall := range toolCalls {
// 		results = append(results, toolCall.Function.Name)
// 	}
//
// 	return results
// }

// getSessionMessages converts the generic messages in 'session' to messages appropriate for a ChatGPT conversation
// history.
// func (c *ChatGPT) getSessionMessages(session *llms.Session) []openai.ChatCompletionMessageParamUnion {
// 	results := make([]openai.ChatCompletionMessageParamUnion, len(session.Messages))
//
// 	for num, msg := range session.Messages {
// 		switch msg.Role {
// 		case llms.RoleAssistant:
// 			assistantMsg := openai.AssistantMessage(msg.Content)
//
// 			if msg.HasToolCalls() {
// 				toolCalls, ok := msg.ToolCallInfo.([]openai.ChatCompletionMessageToolCallUnionParam)
// 				if !ok {
// 					c.logger.Warn("got ToolCallInfo of unexpected type", "type", fmt.Sprintf("%T", msg.ToolCallInfo))
// 				}
//
// 				assistantMsg.OfAssistant.ToolCalls = toolCalls
// 			}
//
// 			results[num] = assistantMsg
// 		case llms.RoleSystem:
// 			results[num] = openai.SystemMessage(msg.Content)
// 		case llms.RoleUser:
// 			results[num] = openai.UserMessage(msg.Content)
// 		case llms.RoleTool:
// 			results[num] = openai.ToolMessage(msg.Content, msg.ToolCallID)
// 		case llms.RoleUnknown:
// 			c.logger.Warn("got message with unknown role", "message", msg.Content)
// 		}
// 	}
//
// 	return results
// }

// func (c *ChatGPT) handleStreamingResponse(
// 	stream *ssestream.Stream[openai.ChatCompletionChunk], msg *llms.Message, chunkChan chan<- *llms.Message,
// ) (openai.ChatCompletionAccumulator, error) {
// 	accumulator := openai.ChatCompletionAccumulator{}
//
// 	for stream.Next() {
// 		chunk := stream.Current()
// 		accumulator.AddChunk(chunk)
//
// 		if _, ok := accumulator.JustFinishedContent(); ok {
// 			c.logger.Debug("got end of content",
// 				"current_delta", chunk.Choices[0].Delta.Content,
// 				"finish_reason", chunk.Choices[0].FinishReason)
// 		}
//
// 		if _, ok := accumulator.JustFinishedToolCall(); ok {
// 			c.logger.Debug("got end of tool call info",
// 				"current_delta", chunk.Choices[0].Delta.Content,
// 				"tool_calls", accumulator.Choices[0].Message.ToolCalls)
// 		}
//
// 		if refusal, ok := accumulator.JustFinishedRefusal(); ok {
// 			return accumulator, fmt.Errorf("%w: %s", llms.ErrPromptRefused, refusal)
// 		}
//
// 		msg = msg.Update(llms.WithChunkContent(chunk.Choices[0].Delta.Content))
//
// 		chunkChan <- msg
//
// 		msg = msg.Update(llms.WithIsInitial(false))
// 	}
//
// 	if err := stream.Err(); err != nil {
// 		return accumulator, fmt.Errorf("%w: streaming: %w", llms.ErrCompletion, err)
// 	}
//
// 	return accumulator, nil
// }
