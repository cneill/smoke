package prompts

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"
)

// TODO: split markdown from JSON
type Prompt interface {
	Name() string
	Markdown() string
	JSONMap() map[string]any
	JSONString() (string, error)
}

func NewSectionsPrompt(name string, opts ...PromptOpt) (Prompt, error) {
	sectionKeys := orderedSections()

	newPrompt := &sectionsPrompt{
		name:        name,
		sectionKeys: sectionKeys,
		sections:    make([]Section, len(sectionKeys)),
	}

	var optErr error

	for idx, opt := range opts {
		newPrompt, optErr = opt(newPrompt)
		if optErr != nil {
			return nil, fmt.Errorf("error with option at index %d: %w", idx, optErr)
		}
	}

	return newPrompt, nil
}

type PromptOpt func(*sectionsPrompt) (*sectionsPrompt, error)

func WithSection(name SectionType, items ...string) PromptOpt {
	return func(prompt *sectionsPrompt) (*sectionsPrompt, error) {
		idx := slices.Index(prompt.sectionKeys, name)
		if idx == -1 {
			return nil, fmt.Errorf("unknown section name: %q", name)
		}

		if section := prompt.sections[idx]; len(section) > 0 {
			return nil, fmt.Errorf("section %q is already populated", name)
		}

		prompt.sections[idx] = items

		return prompt, nil
	}
}

type Section []string

type SectionType string

func (s SectionType) JSONKey() string {
	var result string

	for word := range strings.SplitSeq(string(s), " ") {
		result += strings.ToLower(word)
		result += "_"
	}

	return strings.TrimSuffix(result, "_")
}

// order inspired by Anthropic - https://x.com/mattpocockuk/status/1958179930262356032/photo/1
const (
	SectionTask                SectionType = "Task"
	SectionTone                SectionType = "Tone"
	SectionBackground          SectionType = "Background"
	SectionDescription         SectionType = "Description"
	SectionRules               SectionType = "Rules"
	SectionExamples            SectionType = "Examples"
	SectionConversationHistory SectionType = "Conversation History"
	SectionInstructions        SectionType = "Instructions"
	SectionFormatting          SectionType = "Formatting"
)

func orderedSections() []SectionType {
	return []SectionType{
		SectionTask,
		SectionTone,
		SectionBackground,
		SectionDescription,
		SectionRules,
		SectionExamples,
		SectionConversationHistory,
		SectionInstructions,
		SectionFormatting,
	}
}

type sectionsPrompt struct {
	name string
	// We keep the keys and sections as separate arrays here to ensure the ordering stays consistent. A map would be
	// randomized, defeating the point.
	sectionKeys []SectionType
	sections    []Section
}

func (s *sectionsPrompt) Name() string { return s.name }

func (s *sectionsPrompt) Markdown() string {
	builder := &strings.Builder{}

	for sectionIdx, sectionHeading := range s.sectionKeys {
		if len(s.sections[sectionIdx]) == 0 {
			continue
		}

		builder.WriteString("## ")
		builder.WriteString(string(sectionHeading))
		builder.WriteString("\n\n")

		for _, sectionItem := range s.sections[sectionIdx] {
			builder.WriteString(" * ")
			builder.WriteString(sectionItem)
			builder.WriteByte('\n')
		}

		builder.WriteByte('\n')
	}

	return builder.String()
}

func (s *sectionsPrompt) JSONMap() map[string]any {
	result := map[string]any{}

	for sectionIdx, sectionHeading := range s.sectionKeys {
		if sections := s.sections[sectionIdx]; len(sections) > 0 {
			result[sectionHeading.JSONKey()] = sections
		}
	}

	return result
}

func (s *sectionsPrompt) JSONString() (string, error) {
	jsonMap := s.JSONMap()

	jsonBytes, err := json.Marshal(jsonMap)
	if err != nil {
		return "", fmt.Errorf("invalid JSON: %w", err)
	}

	return string(jsonBytes), nil
}
