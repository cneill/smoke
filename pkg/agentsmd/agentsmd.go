// Package agentsmd handles the loading of AGENTS.md files from <project>/AGENTS.md and $HOME/.config/smoke/AGENTS.md
package agentsmd

import (
	"fmt"
	"strings"
)

type File struct {
	Path     string
	Contents []byte
}

func (a *File) String() string {
	builder := &strings.Builder{}

	fmt.Fprintf(builder, "<<<< %s >>>>\n\n", a.Path)
	builder.Write(a.Contents)

	return builder.String()
}

// Catalog holds all the discovered AGENTS.md files that are relevant for Smoke.
type Catalog []*File

func (c Catalog) String() string {
	if len(c) == 0 {
		return ""
	}

	builder := &strings.Builder{}

	fmt.Fprintf(builder, "<< AGENTS.md file instructions >>")

	for _, agentsMD := range c {
		builder.WriteString(agentsMD.String())
		builder.WriteString("\n=====\n")
	}

	return builder.String()
}
