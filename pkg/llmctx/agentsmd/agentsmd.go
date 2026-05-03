// Package agentsmd handles the loading of AGENTS.md files from <project>/AGENTS.md and $HOME/.config/smoke/AGENTS.md
package agentsmd

import (
	"fmt"
	"strings"
)

type Type string

func (t Type) Upper() string { return strings.ToUpper(string(t)) }

const (
	TypeUser    Type = "user"
	TypeProject Type = "project"
)

type File struct {
	Type     Type
	Path     string
	Contents []byte
}

func (a *File) String() string {
	builder := &strings.Builder{}

	// TODO: add relative path for project-scoped AGENTS.md in future?
	fmt.Fprintf(builder, "< %s AGENTS.md INSTRUCTIONS >\n\n", a.Type.Upper())
	builder.Write(a.Contents)
	fmt.Fprintf(builder, "\n\n< END %s AGENTS.md INSTRUCTIONS >\n", a.Type.Upper())

	return builder.String()
}

// Catalog holds all the discovered AGENTS.md files that are relevant for Smoke.
type Catalog []*File

func (c Catalog) String() string {
	if len(c) == 0 {
		return ""
	}

	builder := &strings.Builder{}

	builder.WriteString("\n< AGENTS.md INSTRUCTIONS >\n\n")
	fmt.Fprintf(builder,
		"The user has defined custom instructions for you. Instructions marked as %q are defined in the user's home "+
			"directory. Instructions marked as %q are specific to this project/git repository. You should prefer "+
			"instructions from the project over general user instructions, as they are more specific.\n\n",
		TypeUser.Upper(), TypeProject.Upper(),
	)

	for _, agentsMD := range c {
		builder.WriteString(agentsMD.String())
		builder.WriteRune('\n')
	}

	builder.WriteString("\n< END AGENTS.md INSTRUCTIONS >\n\n")

	return builder.String()
}
