package smoke

import (
	"fmt"
	"strings"

	"github.com/cneill/smoke/pkg/llmctx/modes"
	"github.com/cneill/smoke/pkg/llmctx/prompts"
)

func (s *Smoke) SystemPrompt(mode modes.Mode) (string, error) {
	builder := &strings.Builder{}

	var basePrompt *prompts.Prompt

	switch mode {
	case modes.ModeWork:
		basePrompt = prompts.WorkSystemPrompt()
	case modes.ModePlanning:
		basePrompt = prompts.PlanningSystemPrompt()
	case modes.ModeReview:
		basePrompt = prompts.ReviewSystemPrompt()
	default:
		return "", fmt.Errorf("no standalone system prompt for mode %q, must be one of %q", mode, modes.SelectableModes())
	}

	builder.WriteString("< SYSTEM PROMPT >\n")
	builder.WriteString(basePrompt.Markdown())
	builder.WriteString("\n< END SYSTEM PROMPT >\n")

	agentsPrompt := s.agentsmdCatalog.String()
	if agentsPrompt != "" {
		builder.WriteString(agentsPrompt)
	}

	return builder.String(), nil
}
