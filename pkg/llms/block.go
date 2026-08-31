package llms

import (
	"bytes"
	"encoding/json"
	"fmt"
)

type BlockType string

const (
	BlockTypeUnknown   BlockType = ""
	BlockTypeText      BlockType = "text"
	BlockTypeImage     BlockType = "image"
	BlockTypeToolCall  BlockType = "tool_call"
	BlockTypeReasoning BlockType = "reasoning"
)

type TextBlock struct {
	Text  string `json:"text"`
	Phase string `json:"phase,omitempty"`
}

type ImageBlock struct {
	Data      []byte `json:"data"`
	MediaType string `json:"media_type"`
}

func (i *ImageBlock) OK() error {
	if i.MediaType == "" {
		return fmt.Errorf("missing media type")
	}

	return nil
}

type ToolCallBlock struct {
	ToolCall ToolCall `json:"tool_call"`
}

type ReasoningBlock struct {
	Summary []string        `json:"summary,omitempty"`
	Detail  ReasoningDetail `json:"detail"`
}

func (r *ReasoningBlock) OK() error {
	return r.Detail.OK()
}

type ReasoningDetailType string

const (
	ReasoningDetailTypeUnknown   ReasoningDetailType = ""
	ReasoningDetailTypeOpenAI    ReasoningDetailType = "openai"
	ReasoningDetailTypeAnthropic ReasoningDetailType = "anthropic"
)

type OpenAIReasoningDetail struct {
	Source           LLMType  `json:"source"`
	ID               string   `json:"id"`
	EncryptedContent string   `json:"encrypted_content,omitempty"`
	Content          []string `json:"content,omitempty"`
	Status           string   `json:"status,omitempty"`
}

type AnthropicReasoningDetail struct {
	Source    LLMType `json:"source"`
	Thinking  string  `json:"thinking,omitempty"`
	Signature string  `json:"signature,omitempty"`
	Data      string  `json:"data,omitempty"`
	Redacted  bool    `json:"redacted,omitempty"`
}

type ReasoningDetail struct {
	OpenAI    *OpenAIReasoningDetail    `json:"openai,omitempty"`
	Anthropic *AnthropicReasoningDetail `json:"anthropic,omitempty"`
}

func (r ReasoningDetail) OK() error {
	switch r.Type() {
	case ReasoningDetailTypeUnknown:
		return fmt.Errorf("unknown reasoning detail type, must supply exactly one of either OpenAI or Anthropic")
	case ReasoningDetailTypeAnthropic:
		if r.Anthropic.Source != LLMTypeClaude {
			return fmt.Errorf("invalid Anthropic reasoning source %q", r.Anthropic.Source)
		}
	case ReasoningDetailTypeOpenAI:
		switch r.OpenAI.Source {
		case LLMTypeChatGPT, LLMTypeGrok, LLMTypeOllama:
			return nil
		default:
			return fmt.Errorf("invalid OpenAI reasoning source %q", r.OpenAI.Source)
		}
	}

	return nil
}

func (r ReasoningDetail) Type() ReasoningDetailType {
	switch {
	case (r.OpenAI == nil) == (r.Anthropic == nil):
		return ReasoningDetailTypeUnknown
	case r.OpenAI != nil:
		return ReasoningDetailTypeOpenAI
	case r.Anthropic != nil:
		return ReasoningDetailTypeAnthropic
	}

	return ReasoningDetailTypeUnknown
}

// TODO: Canonical server-side tool blocks can be added in the future.

type Block struct {
	Text      *TextBlock      `json:"text,omitempty"`
	Image     *ImageBlock     `json:"image,omitempty"`
	ToolCall  *ToolCallBlock  `json:"tool_call,omitempty"`
	Reasoning *ReasoningBlock `json:"reasoning,omitempty"`
}

func (b Block) Type() BlockType {
	candidates := []struct {
		typ     BlockType
		present bool
	}{
		{typ: BlockTypeText, present: b.Text != nil},
		{typ: BlockTypeImage, present: b.Image != nil},
		{typ: BlockTypeToolCall, present: b.ToolCall != nil},
		{typ: BlockTypeReasoning, present: b.Reasoning != nil},
	}

	typ := BlockTypeUnknown

	for _, candidate := range candidates {
		if !candidate.present {
			continue
		}

		if typ != BlockTypeUnknown {
			return BlockTypeUnknown
		}

		typ = candidate.typ
	}

	return typ
}

func (b Block) OK() error {
	switch b.Type() { //nolint:exhaustive
	case BlockTypeUnknown:
		return fmt.Errorf("block must contain exactly one payload")
	case BlockTypeImage:
		return b.Image.OK()
	case BlockTypeReasoning:
		return b.Reasoning.OK()
	}

	return nil
}

func (b Block) MarshalJSON() ([]byte, error) {
	if b.Type() == BlockTypeUnknown {
		return nil, fmt.Errorf("block must contain exactly one payload")
	}

	type canonical Block

	data, err := json.Marshal(struct {
		canonical

		Type BlockType `json:"type"`
	}{
		canonical: canonical(b),
		Type:      b.Type(),
	})
	if err != nil {
		return nil, fmt.Errorf("marshal block: %w", err)
	}

	return data, nil
}

func (b *Block) UnmarshalJSON(data []byte) error {
	type canonical Block

	var raw struct {
		canonical

		Type BlockType `json:"type"`
	}

	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()

	if err := decoder.Decode(&raw); err != nil {
		return fmt.Errorf("unmarshal block: %w", err)
	}

	*b = Block(raw.canonical)
	if b.Type() == BlockTypeUnknown {
		return fmt.Errorf("block must contain exactly one payload")
	}

	if raw.Type != BlockTypeUnknown && raw.Type != b.Type() {
		return fmt.Errorf("block type %q does not match payload type %q", raw.Type, b.Type())
	}

	return nil
}

type Blocks []Block //nolint:recvcheck

func (b Blocks) OK() error {
	for idx, block := range b {
		if err := block.OK(); err != nil {
			return fmt.Errorf("invalid block at index %d: %w", idx, err)
		}
	}

	return nil
}

func (b Blocks) Clone() Blocks {
	result := make(Blocks, len(b))
	for i := range b {
		result[i] = b[i].Clone()
	}

	return result
}

func (b *Blocks) ReplaceType(typ BlockType, replacements Blocks) {
	if b == nil {
		return
	}

	result := make(Blocks, 0, len(*b)+len(replacements))
	inserted := false

	for _, block := range *b {
		if block.Type() == typ {
			if !inserted {
				result = append(result, replacements.Clone()...)
				inserted = true
			}

			continue
		}

		result = append(result, block)
	}

	if !inserted {
		result = append(result, replacements.Clone()...)
	}

	*b = result
}

func TextContentBlock(text string) Block {
	return Block{Text: &TextBlock{Text: text}}
}

func PhasedTextContentBlock(text, phase string) Block {
	return Block{
		Text: &TextBlock{
			Text:  text,
			Phase: phase,
		},
	}
}

func ImageContentBlock(data []byte, mediaType string) Block {
	return Block{
		Image: &ImageBlock{
			Data:      append([]byte(nil), data...),
			MediaType: mediaType,
		},
	}
}

func ToolContentBlock(call ToolCall) Block {
	return Block{
		ToolCall: &ToolCallBlock{
			ToolCall: call.Clone(),
		},
	}
}

func ReasoningContentBlock(reasoning ReasoningBlock) Block {
	return Block{Reasoning: &reasoning}.Clone()
}

// Cloning uses JSON to preserve nested tool argument and reasoning data.

func (b Block) Clone() Block {
	data, err := json.Marshal(b)
	if err != nil {
		panic(err)
	}

	var result Block

	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()

	if err := decoder.Decode(&result); err != nil {
		panic(err)
	}

	return result
}
