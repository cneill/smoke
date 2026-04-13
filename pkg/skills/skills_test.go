package skills_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cneill/smoke/pkg/skills"
)

func TestParseSkillContents_Valid(t *testing.T) {
	t.Parallel()

	contents := `---
name: test-skill
description: A test skill
compatibility: go
metadata:
  author: tester
  version: "1.0"
---
This is the body of the skill.

It has multiple lines.
`
	skill, err := skills.ParseSkillContents(contents)
	require.NoError(t, err)

	assert.Equal(t, "test-skill", skill.Name)
	assert.Equal(t, "A test skill", skill.Description)
	assert.Equal(t, "go", skill.Compatibility)
	assert.Equal(t, map[string]string{"author": "tester", "version": "1.0"}, skill.Metadata)
	assert.Equal(t, "This is the body of the skill.\n\nIt has multiple lines.\n", skill.Body)
}

func TestParseSkillContents_MinimalFields(t *testing.T) {
	t.Parallel()

	contents := `---
name: minimal
description: Just the basics
---
Body here.
`
	skill, err := skills.ParseSkillContents(contents)
	require.NoError(t, err)

	assert.Equal(t, "minimal", skill.Name)
	assert.Equal(t, "Just the basics", skill.Description)
	assert.Empty(t, skill.Compatibility)
	assert.Empty(t, skill.Metadata)
	assert.Equal(t, "Body here.\n", skill.Body)
}

func TestParseSkillContents_MissingName(t *testing.T) {
	t.Parallel()

	contents := `---
description: No name here
---
Body.
`
	_, err := skills.ParseSkillContents(contents)
	require.Error(t, err)
	require.ErrorIs(t, err, skills.ErrMissingField)
	assert.Contains(t, err.Error(), "name")
}

func TestParseSkillContents_MissingDescription(t *testing.T) {
	t.Parallel()

	contents := `---
name: no-desc
---
Body.
`
	_, err := skills.ParseSkillContents(contents)
	require.Error(t, err)
	require.ErrorIs(t, err, skills.ErrMissingField)
	assert.Contains(t, err.Error(), "description")
}

func TestParseSkillContents_NoOpeningDelimiter(t *testing.T) {
	t.Parallel()

	contents := `name: test
description: test
---
Body.
`
	_, err := skills.ParseSkillContents(contents)
	require.Error(t, err)
	require.ErrorIs(t, err, skills.ErrInvalidFrontmatter)
}

func TestParseSkillContents_ContentBeforeOpening(t *testing.T) {
	t.Parallel()

	contents := `some preamble
---
name: test
description: test
---
Body.
`
	_, err := skills.ParseSkillContents(contents)
	require.Error(t, err)
	require.ErrorIs(t, err, skills.ErrInvalidFrontmatter)
}

func TestParseSkillContents_NoClosingDelimiter(t *testing.T) {
	t.Parallel()

	contents := `---
name: test
description: test
Body without closing delimiter.
`
	_, err := skills.ParseSkillContents(contents)
	require.Error(t, err)
	require.ErrorIs(t, err, skills.ErrInvalidFrontmatter)
	assert.Contains(t, err.Error(), "no closing")
}

func TestParseSkillContents_EmptyBody(t *testing.T) {
	t.Parallel()

	contents := `---
name: empty-body
description: Skill with no body
---
`
	skill, err := skills.ParseSkillContents(contents)
	require.NoError(t, err)

	assert.Equal(t, "empty-body", skill.Name)
	assert.Empty(t, skill.Body)
}

func TestParseSkillContents_OnlyDelimiters(t *testing.T) {
	t.Parallel()

	contents := "---\n---\n"
	_, err := skills.ParseSkillContents(contents)
	require.Error(t, err)
	require.ErrorIs(t, err, skills.ErrMissingField)
}

func TestParseSkillContents_LeadingWhitespace(t *testing.T) {
	t.Parallel()

	contents := `  ---
name: indented
description: Starts with spaces
---
Body.
`
	_, err := skills.ParseSkillContents(contents)
	require.Error(t, err)
	require.ErrorIs(t, err, skills.ErrInvalidFrontmatter)
}

func TestParseSkillFile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	skillPath := filepath.Join(dir, "SKILL.md")

	contents := `---
name: file-test
description: Parsed from a file
---
File body content.
`
	require.NoError(t, os.WriteFile(skillPath, []byte(contents), 0o644))

	skill, err := skills.ParseSkillFile(skillPath)
	require.NoError(t, err)

	assert.Equal(t, "file-test", skill.Name)
	assert.Equal(t, "Parsed from a file", skill.Description)
	assert.Equal(t, "File body content.\n", skill.Body)
	assert.Equal(t, skillPath, skill.Source)
}

func TestParseSkillFile_NotFound(t *testing.T) {
	t.Parallel()

	_, err := skills.ParseSkillFile("/nonexistent/SKILL.md")
	require.Error(t, err)
}

func TestCatalog_ByName(t *testing.T) {
	t.Parallel()

	catalog := skills.Catalog{
		{Name: "alpha", Description: "First"},
		{Name: "beta", Description: "Second"},
	}

	found := catalog.ByName("beta")
	require.NotNil(t, found)
	assert.Equal(t, "Second", found.Description)

	assert.Nil(t, catalog.ByName("gamma"))
}

func TestCatalog_Names(t *testing.T) {
	t.Parallel()

	catalog := skills.Catalog{
		{Name: "alpha"},
		{Name: "beta"},
		{Name: "gamma"},
	}

	assert.Equal(t, []string{"alpha", "beta", "gamma"}, catalog.Names())
}

func TestCatalog_NamesEmpty(t *testing.T) {
	t.Parallel()

	catalog := skills.Catalog{}
	assert.Empty(t, catalog.Names())
}

func TestDiscover_ProjectOverridesHome(t *testing.T) {
	homeDir := t.TempDir()
	projectDir := t.TempDir()

	homeSkillDir := filepath.Join(homeDir, ".agents/skills", "my-skill")
	require.NoError(t, os.MkdirAll(homeSkillDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(homeSkillDir, "SKILL.md"), []byte(`---
name: shared-skill
description: Home version
---
Home body.
`), 0o644))

	projectSkillDir := filepath.Join(projectDir, ".agents/skills", "my-skill")
	require.NoError(t, os.MkdirAll(projectSkillDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(projectSkillDir, "SKILL.md"), []byte(`---
name: shared-skill
description: Project version
---
Project body.
`), 0o644))

	t.Setenv("HOME", homeDir)

	catalog := skills.Discover(projectDir)

	require.Len(t, catalog, 1)
	assert.Equal(t, "shared-skill", catalog[0].Name)
	assert.Equal(t, "Project version", catalog[0].Description)
	assert.Equal(t, "Project body.\n", catalog[0].Body)
	assert.Equal(t, filepath.Join(projectSkillDir, "SKILL.md"), catalog[0].Source)
}

func TestDiscover_MergesDistinctSkills(t *testing.T) {
	homeDir := t.TempDir()
	projectDir := t.TempDir()

	homeSkillDir := filepath.Join(homeDir, ".agents/skills", "home-only")
	require.NoError(t, os.MkdirAll(homeSkillDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(homeSkillDir, "SKILL.md"), []byte(`---
name: home-skill
description: From home
---
Home.
`), 0o644))

	projectSkillDir := filepath.Join(projectDir, ".agents/skills", "project-only")
	require.NoError(t, os.MkdirAll(projectSkillDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(projectSkillDir, "SKILL.md"), []byte(`---
name: project-skill
description: From project
---
Project.
`), 0o644))

	t.Setenv("HOME", homeDir)

	catalog := skills.Discover(projectDir)

	require.Len(t, catalog, 2)
	assert.Equal(t, "home-skill", catalog[0].Name)
	assert.Equal(t, "project-skill", catalog[1].Name)
}

func TestDiscover_NoSkillsDirs(t *testing.T) {
	projectDir := t.TempDir()
	homeDir := t.TempDir()

	t.Setenv("HOME", homeDir)

	catalog := skills.Discover(projectDir)
	assert.Empty(t, catalog)
}

func TestDiscover_InvalidSkillSkipped(t *testing.T) {
	projectDir := t.TempDir()
	homeDir := t.TempDir()

	skillDir := filepath.Join(projectDir, ".agents/skills", "bad-skill")
	require.NoError(t, os.MkdirAll(skillDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(`---
name: bad-skill
---
No description.
`), 0o644))

	goodDir := filepath.Join(projectDir, ".agents/skills", "good-skill")
	require.NoError(t, os.MkdirAll(goodDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(goodDir, "SKILL.md"), []byte(`---
name: good-skill
description: This one is valid
---
Good body.
`), 0o644))

	t.Setenv("HOME", homeDir)

	catalog := skills.Discover(projectDir)

	require.Len(t, catalog, 1)
	assert.Equal(t, "good-skill", catalog[0].Name)
	assert.Equal(t, filepath.Join(goodDir, "SKILL.md"), catalog[0].Source)
}
