package grok

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/cneill/hc/v2"
	"github.com/cneill/smoke/pkg/llms"
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

	transport := httpClient.Transport.(*http.Transport)
	transport.IdleConnTimeout = time.Minute * 2
	transport.ResponseHeaderTimeout = time.Minute * 2

	headers := http.Header{}
	headers.Set("Content-Type", "application/json") // TODO: fix this when moving to SSE
	headers.Set("Authorization", "Bearer "+config.APIKey)

	client, err := hc.New(httpClient,
		hc.ClientBaseURL("https://api.x.ai"),
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

type ChatCompletionRequest struct {
	Messages            []*ChatCompletionMessage `json:"messages"`
	MaxCompletionTokens int64                    `json:"max_completion_tokens,omitempty"`
	Model               string                   `json:"model"`
	N                   int64                    `json:"n,omitempty"`                   // number of chat completion choices
	ParallelToolCalls   bool                     `json:"parallel_tool_calls,omitempty"` // set to "false" to only do 1 tool call at a time
	ReasoningEffort     string                   `json:"reasoning_effort,omitempty"`    // "low" or "high"; not supported by Grok-4
	Stream              bool                     `json:"stream,omitempty"`              // stream responses with SSE
	Temperature         float64                  `json:"temperature,omitempty"`
	// ToolChoice - can be used to force the model to use a tool, or never use one
	Tools []*ChatCompletionTool `json:"tools"`
}

type ChatCompletionMessage struct {
	Role       llms.Role                 `json:"role"`
	Content    string                    `json:"content"`
	ToolCalls  []*ChatCompletionToolCall `json:"tool_calls,omitempty"`
	ToolCallID string                    `json:"tool_call_id,omitempty"`

	// only in responses?
	ReasoningContent string `json:"reasoning_content,omitempty"`
	Refusal          string `json:"refusal,omitempty"`
}

type ChatCompletionToolCall struct {
	ID       string                          `json:"id"`
	Index    int64                           `json:"index"`
	Function *ChatCompletionToolCallFunction `json:"function"`
	Type     string                          `json:"type"` // always "function"
}

type ChatCompletionToolCallFunction struct {
	Arguments string `json:"arguments"`
	Name      string `json:"name"`
}

type ChatCompletionTool struct {
	Function *ChatCompletionToolFunction `json:"function"`
}

type ChatCompletionToolFunction struct {
	Description string         `json:"description"`
	Name        string         `json:"name"`
	Parameters  map[string]any `json:"parameters"` // TODO: make this just "any" ?
}

type ChatCompletionResponse struct {
	Choices   []*ChatCompletionChoice
	Created   int64                `json:"created"` // UNIX timestamp
	ID        string               `json:"id"`
	Model     string               `json:"model"`
	Object    string               `json:"object"` // always "chat.completion"
	Citations []string             `json:"citations"`
	Usage     *ChatCompletionUsage `json:"usage"`
}

type ChatCompletionChoice struct {
	FinishReason string `json:"finish_reason"` // "stop" = stop sequence, "length" = tokens, "end_turn"
	Index        int64  `json:"index"`
	// Logprobs
	Message *ChatCompletionMessage `json:"message"` // TODO: make this a separate type?
}

type ChatCompletionUsage struct {
	CompletionTokens        int64                              `json:"completion_tokens"`
	CompletionTokensDetails *ChatCompletionTokensDetails       `json:"completion_tokens_details"`
	NumSourcesUsed          int64                              `json:"num_sources_used"`
	PromptTokens            int64                              `json:"prompt_tokens"`
	PromptTokensDetails     *ChatCompletionPromptTokensDetails `json:"prompt_tokens_details"`
	TotalTokens             int64                              `json:"total_tokens"`
}

type ChatCompletionTokensDetails struct {
	AcceptedPredictionTokens int64 `json:"accepted_prediction_tokens"`
	AudioTokens              int64 `json:"audio_tokens"`
	ReasoningTokens          int64 `json:"reasoning_tokens"`
	RejectedPredictionTokens int64 `json:"rejected_prediction_tokens"`
}

type ChatCompletionPromptTokensDetails struct {
	AudioTokens  int64 `json:"audio_tokens"`
	CachedTokens int64 `json:"cached_tokens"`
	ImageTokens  int64 `json:"image_tokens"`
	TextTokens   int64 `json:"text_tokens"`
}

func (g *Grok) SendSession(ctx context.Context, session *llms.Session) (*llms.Message, error) {
	req := &ChatCompletionRequest{
		Model:               g.config.Model,
		Messages:            g.getSessionMessages(session),
		N:                   1,
		ParallelToolCalls:   false,
		Temperature:         g.config.Temperature,
		MaxCompletionTokens: g.config.MaxTokens,
		Stream:              false, // for now
		// TODO? ReasoningEffort:
		// TODO: Tools:
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

	slog.Debug("got API response", "resp", httpResp)

	resp := &ChatCompletionResponse{}

	_, err = hc.HandleResponse(httpResp,
		hc.JSONResponse(resp),
		hc.AllowedStatusCodes(hc.Status2XX...),
	)
	if err != nil {
		return nil, fmt.Errorf("response handling error: %w", err)
	}

	slog.Debug("got API response body", "resp", resp)

	if len(resp.Choices) == 0 {
		return nil, fmt.Errorf("%w: no choices returned", llms.ErrEmptyResponse)
	}

	if refusal := resp.Choices[0].Message.Refusal; refusal != "" {
		return nil, fmt.Errorf("%w: %s", llms.ErrPromptRefused, refusal)
	}

	msg := llms.NewMessage(
		llms.WithID(resp.ID),
		llms.WithRole(llms.RoleAssistant),
		llms.WithContent(resp.Choices[0].Message.Content),
		llms.WithLLMInfo(g.LLMInfo()),
		llms.WithIsStreamed(false), // TODO: Fix
		// TODO: handle tool calls
	)

	return msg, nil
}

func (g *Grok) HandleToolCalls(msg *llms.Message, session *llms.Session) ([]*llms.Message, error) {
	return nil, nil
}

// func (g *Grok) SendSessionStreaming(ctx context.Context, s *llms.Session, chunkChan chan<- *llms.Message) (*llms.Message, error) {
// 	return nil, nil
// }

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
			// TODO: ToolCalls
		case llms.RoleSystem:
			// results[num] =
			// results[num] = openai.SystemMessage(msg.Content)
		case llms.RoleUser:
			// results[num] = openai.UserMessage(msg.Content)
		case llms.RoleTool:
			// results[num] = openai.ToolMessage(msg.Content, msg.ToolCallID)
		case llms.RoleUnknown:
			g.logger.Warn("got message with unknown role", "message", msg.Content)
		}
	}

	return results
}
