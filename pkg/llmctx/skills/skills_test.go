package skills_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cneill/smoke/pkg/llmctx/skills"
)

func TestParseSkillContents(t *testing.T) { //nolint:funlen
	t.Parallel()

	tests := []struct {
		name        string
		contents    string
		wantSkill   *skills.Skill
		wantErr     error
		errContains string
	}{
		{
			name: "valid_full",
			contents: "---\nname: test-skill\ndescription: A test skill\n" +
				"compatibility: go\nmetadata:\n  author: tester\n  version: \"1.0\"\n" +
				"---\nThis is the body of the skill.\n\nIt has multiple lines.\n",
			wantSkill: &skills.Skill{
				Name:          "test-skill",
				Description:   "A test skill",
				Compatibility: "go",
				Metadata:      map[string]string{"author": "tester", "version": "1.0"},
				Body:          "This is the body of the skill.\n\nIt has multiple lines.\n",
			},
		},
		{
			name:      "minimal_fields",
			contents:  "---\nname: minimal\ndescription: Just the basics\n---\nBody here.\n",
			wantSkill: &skills.Skill{Name: "minimal", Description: "Just the basics", Body: "Body here.\n"},
		},
		{
			name:      "empty_body",
			contents:  "---\nname: empty-body\ndescription: Skill with no body\n---\n",
			wantSkill: &skills.Skill{Name: "empty-body", Description: "Skill with no body", Body: ""},
		},
		{
			name:        "missing_name",
			contents:    "---\ndescription: No name here\n---\nBody.\n",
			wantErr:     skills.ErrMissingField,
			errContains: "name",
		},
		{
			name:        "missing_description",
			contents:    "---\nname: no-desc\n---\nBody.\n",
			wantErr:     skills.ErrMissingField,
			errContains: "description",
		},
		{
			name:     "only_delimiters",
			contents: "---\n---\n",
			wantErr:  skills.ErrMissingField,
		},
		{
			name:     "no_opening_delimiter",
			contents: "name: test\ndescription: test\n---\nBody.\n",
			wantErr:  skills.ErrInvalidFrontmatter,
		},
		{
			name:     "content_before_opening",
			contents: "some preamble\n---\nname: test\ndescription: test\n---\nBody.\n",
			wantErr:  skills.ErrInvalidFrontmatter,
		},
		{
			name:        "no_closing_delimiter",
			contents:    "---\nname: test\ndescription: test\nBody without closing delimiter.\n",
			wantErr:     skills.ErrInvalidFrontmatter,
			errContains: "no closing",
		},
		{
			name:     "leading_whitespace",
			contents: "  ---\nname: indented\ndescription: Starts with spaces\n---\nBody.\n",
			wantErr:  skills.ErrInvalidFrontmatter,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			skill, err := skills.ParseSkillContents(test.contents)
			if test.wantErr == nil {
				require.NoError(t, err)
				require.NotNil(t, skill)
				assert.Equal(t, test.wantSkill.Name, skill.Name)
				assert.Equal(t, test.wantSkill.Description, skill.Description)
				assert.Equal(t, test.wantSkill.Compatibility, skill.Compatibility)
				assert.Equal(t, test.wantSkill.Metadata, skill.Metadata)
				assert.Equal(t, test.wantSkill.Body, skill.Body)
			} else {
				require.ErrorIs(t, err, test.wantErr)

				if test.errContains != "" {
					assert.Contains(t, err.Error(), test.errContains)
				}
			}
		})
	}
}

func TestParseSkillFile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	tests := []struct {
		name      string
		setup     func(t *testing.T) string
		wantSkill *skills.Skill
		wantErr   bool
	}{
		{
			name: "valid_file",
			setup: func(t *testing.T) string {
				t.Helper()

				skillPath := filepath.Join(dir, "SKILL.md")
				content := []byte("---\nname: file-test\ndescription: Parsed from a file\n---\nFile body content.\n")
				require.NoError(t, os.WriteFile(skillPath, content, 0o644))

				return skillPath
			},
			wantSkill: &skills.Skill{
				Name:        "file-test",
				Description: "Parsed from a file",
				Body:        "File body content.\n",
			},
		},
		{
			name: "not_found",
			setup: func(t *testing.T) string {
				t.Helper()
				return "/nonexistent/SKILL.md"
			},
			wantErr: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			path := test.setup(t)
			skill, err := skills.ParseSkillFile(path)

			if test.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, skill)
			assert.Equal(t, test.wantSkill.Name, skill.Name)
			assert.Equal(t, test.wantSkill.Description, skill.Description)
			assert.Equal(t, test.wantSkill.Body, skill.Body)
			assert.Equal(t, path, skill.Source)
		})
	}
}

func TestCatalog_ByName(t *testing.T) {
	t.Parallel()

	catalog := skills.Catalog{
		{Name: "alpha", Description: "First"},
		{Name: "beta", Description: "Second"},
	}

	tests := []struct {
		name      string
		query     string
		wantFound bool
		wantDesc  string
	}{
		{name: "found", query: "beta", wantFound: true, wantDesc: "Second"},
		{name: "not_found", query: "gamma", wantFound: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			result := catalog.ByName(test.query)
			if test.wantFound {
				require.NotNil(t, result)
				assert.Equal(t, test.wantDesc, result.Description)
			} else {
				assert.Nil(t, result)
			}
		})
	}
}

func TestCatalog_Names(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		catalog   skills.Catalog
		wantNames []string
	}{
		{
			name:      "multiple_skills",
			catalog:   skills.Catalog{{Name: "alpha"}, {Name: "beta"}, {Name: "gamma"}},
			wantNames: []string{"alpha", "beta", "gamma"},
		},
		{
			name:      "empty_catalog",
			catalog:   skills.Catalog{},
			wantNames: []string{},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, test.wantNames, test.catalog.Names())
		})
	}
}

func TestDiscover(t *testing.T) { //nolint:funlen
	tests := []struct {
		name  string
		setup func(t *testing.T, homeDir, projectDir string)
		check func(t *testing.T, catalog skills.Catalog)
	}{
		{
			name: "project_overrides_home",
			setup: func(t *testing.T, homeDir, projectDir string) {
				t.Helper()

				homeSkillDir := filepath.Join(homeDir, ".agents/skills", "my-skill")
				require.NoError(t, os.MkdirAll(homeSkillDir, 0o755))
				require.NoError(t, os.WriteFile(filepath.Join(homeSkillDir, "SKILL.md"),
					[]byte("---\nname: shared-skill\ndescription: Home version\n---\nHome body.\n"), 0o644))

				projectSkillDir := filepath.Join(projectDir, ".agents/skills", "my-skill")
				require.NoError(t, os.MkdirAll(projectSkillDir, 0o755))
				require.NoError(t, os.WriteFile(filepath.Join(projectSkillDir, "SKILL.md"),
					[]byte("---\nname: shared-skill\ndescription: Project version\n---\nProject body.\n"), 0o644))
			},
			check: func(t *testing.T, catalog skills.Catalog) {
				t.Helper()
				require.Len(t, catalog, 1)
				assert.Equal(t, "shared-skill", catalog[0].Name)
				assert.Equal(t, "Project version", catalog[0].Description)
				assert.Equal(t, "Project body.\n", catalog[0].Body)
			},
		},
		{
			name: "merges_distinct_skills",
			setup: func(t *testing.T, homeDir, projectDir string) {
				t.Helper()

				homeSkillDir := filepath.Join(homeDir, ".agents/skills", "home-only")
				require.NoError(t, os.MkdirAll(homeSkillDir, 0o755))
				require.NoError(t, os.WriteFile(filepath.Join(homeSkillDir, "SKILL.md"),
					[]byte("---\nname: home-skill\ndescription: From home\n---\nHome.\n"), 0o644))

				projectSkillDir := filepath.Join(projectDir, ".agents/skills", "project-only")
				require.NoError(t, os.MkdirAll(projectSkillDir, 0o755))
				require.NoError(t, os.WriteFile(filepath.Join(projectSkillDir, "SKILL.md"),
					[]byte("---\nname: project-skill\ndescription: From project\n---\nProject.\n"), 0o644))
			},
			check: func(t *testing.T, catalog skills.Catalog) {
				t.Helper()
				require.Len(t, catalog, 2)
				assert.Equal(t, "home-skill", catalog[0].Name)
				assert.Equal(t, "project-skill", catalog[1].Name)
			},
		},
		{
			name:  "no_skills_dirs",
			setup: func(t *testing.T, _, _ string) { t.Helper() },
			check: func(t *testing.T, catalog skills.Catalog) {
				t.Helper()
				assert.Empty(t, catalog)
			},
		},
		{
			name: "invalid_skill_skipped",
			setup: func(t *testing.T, _, projectDir string) {
				t.Helper()

				badDir := filepath.Join(projectDir, ".agents/skills", "bad-skill")
				require.NoError(t, os.MkdirAll(badDir, 0o755))
				require.NoError(t, os.WriteFile(filepath.Join(badDir, "SKILL.md"),
					[]byte("---\nname: bad-skill\n---\nNo description.\n"), 0o644))

				goodDir := filepath.Join(projectDir, ".agents/skills", "good-skill")
				require.NoError(t, os.MkdirAll(goodDir, 0o755))
				require.NoError(t, os.WriteFile(filepath.Join(goodDir, "SKILL.md"),
					[]byte("---\nname: good-skill\ndescription: This one is valid\n---\nGood body.\n"), 0o644))
			},
			check: func(t *testing.T, catalog skills.Catalog) {
				t.Helper()
				require.Len(t, catalog, 1)
				assert.Equal(t, "good-skill", catalog[0].Name)
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			homeDir := t.TempDir()
			projectDir := t.TempDir()
			test.setup(t, homeDir, projectDir)
			t.Setenv("HOME", homeDir)
			test.check(t, skills.Discover(projectDir))
		})
	}
}
