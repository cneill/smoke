package skills

import (
	"context"
	"fmt"
	"strings"

	"github.com/cneill/smoke/pkg/llmctx/skills"
	"github.com/cneill/smoke/pkg/tools"
)

const (
	ParamName = "name"
)

// ActivateSkill is a tool that lets the LLM activate a skill by name, returning the skill's markdown body content as
// the tool result. The tool's description includes a catalog of available skills for progressive disclosure.
type ActivateSkill struct {
	catalog skills.Catalog
}

func New(_, _ string) (tools.Tool, error) {
	return &ActivateSkill{}, nil
}

func (a *ActivateSkill) SetSkillCatalog(catalog skills.Catalog) {
	a.catalog = catalog
}

func (a *ActivateSkill) Name() string { return tools.NameActivateSkill }

func (a *ActivateSkill) Description() string {
	sb := strings.Builder{}
	sb.WriteString("Activate a skill by name, loading its instructions into the conversation. Use this when a task " +
		"would benefit from specialized instructions provided by one of the available skills, or when the user " +
		"requests it directly.\n\n")

	if len(a.catalog) == 0 {
		sb.WriteString("No skills are currently available.")
	} else {
		sb.WriteString("## Available Skills\n\n")

		for _, skill := range a.catalog {
			fmt.Fprintf(&sb, "* `%s`: %s\n", skill.Name, skill.Description)
		}
	}

	examples := tools.CollectExamples(a.Examples()...)
	sb.WriteString(examples)

	return sb.String()
}

func (a *ActivateSkill) Examples() tools.Examples {
	return tools.Examples{
		{
			Description: `Activate a skill called "my-skill"`,
			Args:        tools.Args{ParamName: "my-skill"},
		},
	}
}

func (a *ActivateSkill) Params() tools.Params {
	return tools.Params{
		{
			Key:              ParamName,
			Description:      "The name of the skill to activate",
			Type:             tools.ParamTypeString,
			Required:         true,
			EnumStringValues: a.catalog.Names(),
		},
	}
}

func (a *ActivateSkill) Run(_ context.Context, args tools.Args) (*tools.Output, error) {
	name := args.GetString(ParamName)
	if name == nil {
		return nil, fmt.Errorf("%w: no skill name supplied", tools.ErrArguments)
	}

	skill := a.catalog.ByName(*name)
	if skill == nil {
		return nil, fmt.Errorf("%w: unknown skill %q", tools.ErrArguments, *name)
	}

	outputText := skill.Body
	if skill.Body == "" {
		outputText = fmt.Sprintf("Skill %q activated, but it has no body content.", *name)
	}

	return &tools.Output{Text: outputText}, nil
}
