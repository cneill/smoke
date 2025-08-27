package grok

type StreamingAccumulator struct {
	Message        *ChatCompletionMessage
	Usage          *ChatCompletionUsage
	ToolCallsAccum map[int]*ChatCompletionToolCall // accumulate tool calls by index
	Refusal        string
}

func NewStreamingAccumulator() *StreamingAccumulator {
	return &StreamingAccumulator{
		Message:        &ChatCompletionMessage{},
		Usage:          &ChatCompletionUsage{},
		ToolCallsAccum: make(map[int]*ChatCompletionToolCall),
	}
}

func (acc *StreamingAccumulator) Accumulate(chunk *ChatCompletionChunk) {
	if chunk == nil || len(chunk.Choices) == 0 {
		return
	}

	delta := chunk.Choices[0].Delta
	if delta == nil {
		return
	}

	// Accumulate content
	if delta.Content != "" {
		acc.Message.Content += delta.Content
	}

	// Accumulate tool calls
	for _, toolCall := range delta.ToolCalls {
		if toolCall.Index == -1 {
			continue // some deltas have index -1, skip them
		}

		if existing, ok := acc.ToolCallsAccum[int(toolCall.Index)]; ok {
			// Accumulate into existing
			if toolCall.Function.Name != "" {
				existing.Function.Name += toolCall.Function.Name
			}
			if toolCall.Function.Arguments != "" {
				existing.Function.Arguments += toolCall.Function.Arguments
			}
			if toolCall.Type != "" {
				existing.Type = toolCall.Type
			}
			if toolCall.ID != "" {
				existing.ID = toolCall.ID
			}
		} else {
			// New tool call
			acc.ToolCallsAccum[int(toolCall.Index)] = &ChatCompletionToolCall{
				ID:    toolCall.ID,
				Index: toolCall.Index,
				Type:  toolCall.Type,
				Function: &ChatCompletionToolCallFunction{
					Name:      toolCall.Function.Name,
					Arguments: toolCall.Function.Arguments,
				},
			}
		}
	}

	// Accumulate usage (last usage wins)
	if chunk.Usage != nil {
		acc.Usage = chunk.Usage
	}

	// Set refusal if present
	if delta.Refusal != "" {
		acc.Refusal = delta.Refusal
	}
}

func (acc *StreamingAccumulator) Finalize() {
	// Collect accumulated tool calls into the message
	for _, toolCall := range acc.ToolCallsAccum {
		acc.Message.ToolCalls = append(acc.Message.ToolCalls, toolCall)
	}
}

func (acc *StreamingAccumulator) JustFinishedRefusal() (string, bool) {
	if acc.Refusal != "" {
		return acc.Refusal, true
	}
	return "", false
}
