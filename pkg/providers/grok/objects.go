package grok

import "github.com/cneill/smoke/pkg/llms"

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
	Type     string                      `json:"type"` // always "function"
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

type ChatCompletionChunk struct {
	ID      string                       `json:"id"`
	Object  string                       `json:"object"` // "chat.completion.chunk"
	Created int64                        `json:"created"`
	Model   string                       `json:"model"`
	Choices []*ChatCompletionChunkChoice `json:"choices"`
	Usage   *ChatCompletionUsage         `json:"usage,omitempty"`
}

type ChatCompletionChunkChoice struct {
	Index        int64                `json:"index"`
	Delta        *ChatCompletionDelta `json:"delta"`
	FinishReason string               `json:"finish_reason,omitempty"`
}

type ChatCompletionDelta struct {
	Role      llms.Role                 `json:"role,omitempty"`
	Content   string                    `json:"content,omitempty"`
	ToolCalls []*ChatCompletionToolCall `json:"tool_calls,omitempty"`
	Refusal   string                    `json:"refusal,omitempty"`
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
