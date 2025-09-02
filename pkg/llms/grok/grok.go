package grok

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/cneill/hc/v2"
	"github.com/cneill/smoke/pkg/llms"
	"github.com/cneill/smoke/pkg/tools"
)

const (
	API_URL = "https://api.x.ai" //nolint:revive
)

type Grok struct {
	config *llms.Config
	logger *slog.Logger
	client *hc.HC
}

func configOK(config *llms.Config) error {
	if err := config.OK(); err != nil {
		return fmt.Errorf("base LLM config error: %w", err)
	}

	if config.Temperature < 0 || config.Temperature > 2 {
		return fmt.Errorf("Grok temperature must be between 0 and 2")
	}

	return nil
}

func New(config *llms.Config) (llms.LLM, error) {
	if err := configOK(config); err != nil {
		return nil, fmt.Errorf("error with Grok options: %w", err)
	}

	httpClient := hc.DefaultClient()
	httpClient.Timeout = time.Minute * 5

	transport, ok := httpClient.Transport.(*http.Transport)
	if ok {
		transport.IdleConnTimeout = time.Minute * 2
		transport.ResponseHeaderTimeout = time.Minute * 2
	}

	headers := http.Header{}
	headers.Set("Content-Type", "application/json") // TODO: fix this when moving to SSE
	headers.Set("Authorization", "Bearer "+config.APIKey)

	client, err := hc.New(httpClient,
		hc.ClientBaseURL(API_URL),
		hc.GlobalHeaders(headers),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to set up API client: %w", err)
	}

	grok := &Grok{
		config: config,
		logger: slog.Default().WithGroup(llms.LLMTypeGrok),
		client: client,
	}

	return grok, nil
}

func (g *Grok) LLMInfo() *llms.LLMInfo {
	return &llms.LLMInfo{
		Type:      llms.LLMTypeGrok,
		ModelName: g.config.Model,
	}
}

func (g *Grok) RequiresSessionSystem() bool { return true }

// TODO: handle retries/rate limits
func (g *Grok) SendSession(ctx context.Context, session *llms.Session) (*llms.Message, error) {
	req := &ChatCompletionRequest{
		Model:               g.config.Model,
		Messages:            g.getSessionMessages(session),
		N:                   1,
		ParallelToolCalls:   false,
		Temperature:         g.config.Temperature,
		MaxCompletionTokens: g.config.MaxTokens,
		Stream:              false,
		Tools:               g.completionTools(session),
		// TODO? ReasoningEffort:
	}

	httpResp, err := g.client.Do(
		hc.Context(ctx),
		hc.Post,
		hc.JSONRequest(req),
		hc.Path("/v1/chat/completions"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to perform chat completion request: %w", err)
	}

	g.logger.Debug("got API response", "status_code", httpResp.StatusCode, "status", httpResp.Status)

	if httpResp.StatusCode != http.StatusOK {
		buf := &bytes.Buffer{}
		limitedReader := io.LimitReader(httpResp.Body, 1024*2014)
		_, _ = buf.ReadFrom(limitedReader)

		return nil, fmt.Errorf("response error: %d %s, body: %s", httpResp.StatusCode, httpResp.Status, buf.String())
	}

	resp := &ChatCompletionResponse{}
	buf := &bytes.Buffer{}

	_, err = hc.HandleResponse(httpResp,
		hc.CopyRaw(buf),
		hc.JSONResponse(resp),
		hc.AllowedStatusCodes(hc.Status2XX...),
	)
	if err != nil {
		slog.Debug("full response", "body", buf.String())
		return nil, fmt.Errorf("response handling error: %w", err)
	}

	g.logger.Debug("got API response body", "resp", resp)

	if len(resp.Choices) == 0 {
		return nil, fmt.Errorf("%w: no choices returned", llms.ErrEmptyResponse)
	}

	if refusal := resp.Choices[0].Message.Refusal; refusal != "" {
		return nil, fmt.Errorf("%w: %s", llms.ErrPromptRefused, refusal)
	}

	if resp.Usage != nil {
		session.UpdateUsage(resp.Usage.PromptTokens, resp.Usage.CompletionTokens)
	}

	msg := llms.NewMessage(
		llms.WithID(resp.ID),
		llms.WithRole(llms.RoleAssistant),
		llms.WithContent(resp.Choices[0].Message.Content),
		llms.WithLLMInfo(g.LLMInfo()),
		llms.WithIsStreamed(false),
		llms.WithToolsCalled(g.getToolCallNames(resp.Choices[0].Message.ToolCalls)...),
		llms.WithToolCallInfo(resp.Choices[0].Message.ToolCalls),
	)

	return msg, nil
}

func (g *Grok) HandleToolCalls(ctx context.Context, msg *llms.Message, session *llms.Session) ([]*llms.Message, error) {
	if !msg.HasToolCalls() {
		return nil, llms.ErrNoToolCalls
	}

	toolCalls, ok := msg.ToolCallInfo.([]*ChatCompletionToolCall)
	if !ok {
		return nil, fmt.Errorf(
			"got unexpected type for ToolCallInfo (expecting []*grok.ChatCompletionToolCall): %T",
			msg.ToolCallInfo)
	}

	results := make([]*llms.Message, len(toolCalls))

	for toolCallNum, toolCall := range toolCalls {
		name := toolCall.Function.Name

		var (
			content     string
			toolCallErr error
		)

		params, err := session.Tools.Params(name)
		if err != nil {
			return nil, fmt.Errorf("failed to get params for tool %q: %w", name, err)
		}

		args, err := tools.GetArgs([]byte(toolCall.Function.Arguments), params)
		if err != nil {
			return nil, fmt.Errorf("failed to get args for tool %q: %w", name, err)
		}

		output, err := session.Tools.CallTool(ctx, name, args)
		if err != nil {
			g.logger.Error("failed to call tool", "tool_name", name, "error", err)
			toolCallErr = fmt.Errorf("failed to call tool %q: %w", name, err)
			content = toolCallErr.Error()
		} else {
			content = output
		}

		toolCallResultMsg := g.newMessage(
			llms.WithRole(llms.RoleTool),
			llms.WithIsChunk(false),
			llms.WithIsStreamed(false),
			llms.WithToolCallID(toolCall.ID),
			llms.WithToolCallArgs(args),
			llms.WithToolsCalled(toolCall.Function.Name),
			llms.WithContent(content),
			llms.WithError(err),
		)

		results[toolCallNum] = toolCallResultMsg
	}

	g.logger.Debug("returning tool call results", "results", results, "num", len(results))

	return results, nil
}

func (g *Grok) SendSessionStreaming(ctx context.Context, session *llms.Session, chunkChan chan<- *llms.Message) (*llms.Message, error) {
	defer close(chunkChan)

	req := &ChatCompletionRequest{
		Model:               g.config.Model,
		Messages:            g.getSessionMessages(session),
		N:                   1,
		ParallelToolCalls:   false,
		Temperature:         g.config.Temperature,
		MaxCompletionTokens: g.config.MaxTokens,
		Stream:              true,
		Tools:               g.completionTools(session),
		// TODO? ReasoningEffort:
	}

	httpResp, err := g.client.Do(
		hc.Context(ctx),
		hc.Post,
		hc.JSONRequest(req),
		hc.Path("/v1/chat/completions"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to perform streaming chat completion request: %w", err)
	}

	defer httpResp.Body.Close()

	g.logger.Debug("got streaming API response", "status_code", httpResp.StatusCode, "status", httpResp.Status)

	if httpResp.StatusCode != http.StatusOK {
		buf := &bytes.Buffer{}
		limitedReader := io.LimitReader(httpResp.Body, 1024*2048)
		_, _ = buf.ReadFrom(limitedReader)

		return nil, fmt.Errorf("streaming response error: %d %s, body: %s", httpResp.StatusCode, httpResp.Status, buf.String())
	}

	msg := g.newMessage(
		llms.WithRole(llms.RoleAssistant),
		llms.WithIsStreamed(true),
		llms.WithIsInitial(true),
		llms.WithIsChunk(true),
	)

	accumulator := NewStreamingAccumulator()
	first := true

	scanner := bufio.NewScanner(httpResp.Body)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}

		data := strings.TrimPrefix(line, "data: ")
		if data == "" || data == "[DONE]" {
			continue
		}

		chunk := &ChatCompletionChunk{}
		if err := json.Unmarshal([]byte(data), chunk); err != nil {
			return nil, fmt.Errorf("failed to unmarshal streaming chunk: %w", err)
		}

		if !first && msg.IsInitial {
			msg = msg.Update(llms.WithIsInitial(false))
		}

		accumulator.Accumulate(chunk)

		if refusal, ok := accumulator.JustFinishedRefusal(); ok {
			return nil, fmt.Errorf("%w: %s", llms.ErrPromptRefused, refusal)
		}

		if len(chunk.Choices) == 0 {
			continue
		}

		if chunk.Choices[0].Delta != nil && chunk.Choices[0].Delta.Content != "" {
			msg = msg.Update(llms.WithChunkContent(chunk.Choices[0].Delta.Content))
		}

		if msg.Content == "" {
			continue
		}

		slog.Debug("got a real chunk", "content", chunk.Choices[0].Delta.Content)

		chunkChan <- msg

		first = false
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading streaming response: %w", err)
	}

	accumulator.Finalize()

	session.UpdateUsage(accumulator.Usage.PromptTokens, accumulator.Usage.CompletionTokens)

	msg = msg.Update(
		llms.WithContent(accumulator.Message.Content),
		llms.WithToolsCalled(g.getToolCallNames(accumulator.Message.ToolCalls)...),
		llms.WithToolCallInfo(accumulator.Message.ToolCalls),
		llms.WithIsInitial(false),
		llms.WithIsChunk(false),
		llms.WithIsFinalized(true),
	)

	return msg, nil
}

func (g *Grok) completionTools(session *llms.Session) []*ChatCompletionTool {
	sessionTools := session.Tools.GetTools()
	results := make([]*ChatCompletionTool, len(sessionTools))

	for toolNum, tool := range sessionTools {
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

		toolDef := &ChatCompletionTool{
			Function: &ChatCompletionToolFunction{
				Description: tool.Description(),
				Name:        tool.Name(),
				Parameters: map[string]any{
					"type":       "object",
					"required":   requiredKeys,
					"properties": properties,
				},
			},
			Type: "function",
		}

		results[toolNum] = toolDef
	}

	return results
}

func (g *Grok) getToolCallNames(toolCalls []*ChatCompletionToolCall) []string {
	results := []string{}
	for _, toolCall := range toolCalls {
		results = append(results, toolCall.Function.Name)
	}

	return results
}

func (g *Grok) newMessage(opts ...llms.MessageOpt) *llms.Message {
	opts = append([]llms.MessageOpt{llms.WithLLMInfo(g.LLMInfo())}, opts...)
	return llms.NewMessage(opts...)
}

func (g *Grok) getSessionMessages(session *llms.Session) []*ChatCompletionMessage {
	results := make([]*ChatCompletionMessage, len(session.Messages))

	for num, msg := range session.Messages {
		results[num] = &ChatCompletionMessage{
			Role:       msg.Role,
			Content:    msg.Content,
			ToolCallID: msg.ToolCallID,
		}
		switch msg.Role {
		case llms.RoleAssistant:
			if msg.HasToolCalls() {
				if toolCalls, ok := msg.ToolCallInfo.([]*ChatCompletionToolCall); ok {
					results[num].ToolCalls = toolCalls
				} else {
					g.logger.Warn("got ToolCallInfo of unexpected type", "type", fmt.Sprintf("%T", msg.ToolCallInfo))
				}
			}
		case llms.RoleSystem:
		case llms.RoleUser:
		case llms.RoleTool:
		case llms.RoleUnknown:
			g.logger.Warn("got message with unknown role", "message", msg.Content)
		}
	}

	return results
}
