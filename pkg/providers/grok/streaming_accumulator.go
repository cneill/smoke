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

func (s *StreamingAccumulator) Accumulate(chunk *ChatCompletionChunk) {
	if chunk == nil || len(chunk.Choices) == 0 {
		return
	}

	delta := chunk.Choices[0].Delta
	if delta == nil {
		return
	}

	if delta.Content != "" {
		s.Message.Content += delta.Content
	}

	s.accumulateToolCalls(delta)

	// Accumulate usage (last usage wins)
	if chunk.Usage != nil {
		s.Usage = chunk.Usage
	}

	// Set refusal if present
	if delta.Refusal != "" {
		s.Refusal = delta.Refusal
	}
}

func (s *StreamingAccumulator) Finalize() {
	// Collect accumulated tool calls into the message
	for _, toolCall := range s.ToolCallsAccum {
		s.Message.ToolCalls = append(s.Message.ToolCalls, toolCall)
	}
}

func (s *StreamingAccumulator) JustFinishedRefusal() (string, bool) {
	if s.Refusal != "" {
		return s.Refusal, true
	}

	return "", false
}

func (s *StreamingAccumulator) accumulateToolCalls(delta *ChatCompletionDelta) {
	for _, toolCall := range delta.ToolCalls {
		if toolCall.Index == -1 {
			continue // some deltas have index -1, skip them
		}

		existing, ok := s.ToolCallsAccum[int(toolCall.Index)]

		if !ok {
			s.ToolCallsAccum[int(toolCall.Index)] = &ChatCompletionToolCall{
				ID:    toolCall.ID,
				Index: toolCall.Index,
				Type:  toolCall.Type,
				Function: &ChatCompletionToolCallFunction{
					Name:      toolCall.Function.Name,
					Arguments: toolCall.Function.Arguments,
				},
			}

			return
		}

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
	}
}
