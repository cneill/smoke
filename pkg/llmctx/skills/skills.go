// Package skills handles discovery and parsing of skill definitions from SKILL.md files.
// Skills are loaded from two locations: a project-level directory (<project>/.agents/skills) and a user-level
// directory ($HOME/.agents/skills). Each skill lives in its own subdirectory and is defined by a SKILL.md file
// containing YAML frontmatter and a markdown body.
package skills

import (
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"

	"gopkg.in/yaml.v3"
)

const skillFileName = "SKILL.md"

var (
	ErrInvalidFrontmatter = errors.New("invalid frontmatter")
	ErrMissingField       = errors.New("missing required field")
)

// Skill represents a single parsed skill definition loaded from a SKILL.md file.
type Skill struct {
	// Name is the unique identifier for the skill (required).
	Name string `yaml:"name"`
	// Description explains what the skill does (required).
	Description string `yaml:"description"`
	// Compatibility is an optional string describing environmental requirements.
	Compatibility string `yaml:"compatibility,omitempty"`
	// Metadata holds optional key-value pairs associated with the skill.
	Metadata map[string]string `yaml:"metadata,omitempty"`
	// Body is the markdown content after the closing frontmatter delimiter.
	Body string `yaml:"-"`
	// Source is the file path the skill was loaded from.
	Source string `yaml:"-"`
}

// OK validates that the required fields are present.
func (s *Skill) OK() error {
	if s.Name == "" {
		return fmt.Errorf("%w: name", ErrMissingField)
	}

	if s.Description == "" {
		return fmt.Errorf("%w: description", ErrMissingField)
	}

	return nil
}

// Catalog is an ordered collection of skills.
type Catalog []*Skill

// ByName returns the skill with the given name, or nil if not found.
func (c Catalog) ByName(name string) *Skill {
	for _, skill := range c {
		if skill.Name == name {
			return skill
		}
	}

	return nil
}

// Names returns a slice of all skill names in the catalog.
func (c Catalog) Names() []string {
	results := make([]string, len(c))
	for i, skill := range c {
		results[i] = skill.Name
	}

	return results
}

func (c Catalog) Completer() func(string) []string {
	return func(input string) []string {
		results := []string{}

		for _, skill := range c {
			if strings.HasPrefix(skill.Name, input) || input == "" {
				description := skill.Description
				if len(description) > 50 {
					description = description[0:50] + "..."
				}

				completionText := fmt.Sprintf("%s - %s", skill.Name, description)
				results = append(results, completionText)
			}
		}

		slices.Sort(results)

		return results
	}
}

// ParseSkillFile reads a SKILL.md file at the given path and returns a parsed Skill.
// The file must start with a "---" frontmatter delimiter, contain valid YAML, and end the frontmatter with another
// "---" delimiter. Everything after the closing delimiter is stored as the skill Body.
func ParseSkillFile(path string) (*Skill, error) {
	contents, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read skill file %q: %w", path, err)
	}

	skill, err := ParseSkillContents(string(contents))
	if err != nil {
		return nil, fmt.Errorf("failed to parse skill file %q: %w", path, err)
	}

	skill.Source = path

	return skill, nil
}

// ParseSkillContents parses the raw string contents of a SKILL.md file into a Skill.
func ParseSkillContents(contents string) (*Skill, error) {
	const delimiter = "---"

	// The file must start with the frontmatter opening delimiter.
	if !strings.HasPrefix(contents, delimiter) {
		return nil, fmt.Errorf("%w: file must start with %q", ErrInvalidFrontmatter, delimiter)
	}

	nlIdx := strings.Index(contents, "\n")
	if nlIdx == -1 {
		return nil, fmt.Errorf("%w: no frontmatter content after %q header", ErrInvalidFrontmatter, delimiter)
	}

	trailing := contents[len(delimiter):nlIdx]

	if strings.TrimSpace(trailing) != "" {
		return nil, fmt.Errorf("%w: invalid contents after %q in frontmatter header: %q", ErrInvalidFrontmatter, delimiter, trailing)
	}

	afterOpener := contents[nlIdx:]

	// Find the closing delimiter. We look for "---" at the start of a line.
	closingIdx := -1

	lines := strings.SplitAfter(afterOpener, "\n")
	charCount := 0

	for _, line := range lines {
		stripped := strings.TrimLeft(line, " \t")
		after, ok := strings.CutPrefix(stripped, delimiter)

		if ok {
			// Ensure the rest of the line (after "---") is only whitespace or empty.
			rest := strings.TrimSpace(after)

			if rest == "" {
				closingIdx = charCount
				break
			}
		}

		charCount += len(line)
	}

	if closingIdx < 0 {
		return nil, fmt.Errorf("%w: no closing %q found", ErrInvalidFrontmatter, delimiter)
	}

	yamlContent := afterOpener[:closingIdx]
	bodyStart := closingIdx + len(delimiter)

	// Find where the closing delimiter line actually ends so we can grab the body.
	remaining := afterOpener[bodyStart:]
	if idx := strings.Index(remaining, "\n"); idx >= 0 {
		remaining = remaining[idx+1:]
	} else {
		remaining = ""
	}

	skill := &Skill{}
	if err := yaml.Unmarshal([]byte(yamlContent), skill); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvalidFrontmatter, err)
	}

	if err := skill.OK(); err != nil {
		return nil, err
	}

	skill.Body = remaining

	return skill, nil
}
