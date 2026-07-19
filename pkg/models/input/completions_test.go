package input_test

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/cneill/smoke/pkg/fs"
	"github.com/cneill/smoke/pkg/llmctx/skills"
	"github.com/cneill/smoke/pkg/models/input"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestCompletionState(
	t *testing.T,
	commandFn func(string) []string,
	skillFn func(string) []*skills.Skill,
	pathFn func(string) []fs.PathMatch,
) *input.CompletionState {
	t.Helper()

	if commandFn == nil {
		commandFn = func(string) []string { return nil }
	}

	if skillFn == nil {
		skillFn = func(string) []*skills.Skill { return nil }
	}

	if pathFn == nil {
		pathFn = func(string) []fs.PathMatch { return nil }
	}

	cs, err := input.NewCompletionState(80, commandFn, skillFn, pathFn)
	require.NoError(t, err)

	return cs
}

func keyRunes(s string) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
}

func matchLabels(matches []input.Match) []string {
	out := make([]string, len(matches))
	for i, m := range matches {
		out[i] = m.Label
		if out[i] == "" {
			out[i] = m.Value
		}
	}

	return out
}

func matchValues(matches []input.Match) []string {
	out := make([]string, len(matches))
	for i, m := range matches {
		out[i] = m.Value
	}

	return out
}

func TestEmptyKeyDoesNotStartCompletion(t *testing.T) {
	t.Parallel()

	cs := newTestCompletionState(t, nil, nil, nil)
	result := cs.HandleKey(tea.KeyMsg{}, "")

	assert.False(t, result.Consume)
	assert.False(t, cs.InCompletion())
}

func TestPathCompletionRequiresExtraCharacter(t *testing.T) {
	t.Parallel()

	pathFn := func(query string) []fs.PathMatch {
		if query == "d" || query == "do" {
			return []fs.PathMatch{{Path: "docs/", IsDir: true}}
		}

		return nil
	}
	cs := newTestCompletionState(t, nil, nil, pathFn)

	result := cs.HandleKey(keyRunes("@"), "")
	handled := result.Consume
	assert.False(t, handled)
	assert.True(t, cs.InPathCompletion())
	assert.False(t, cs.PopupActive())

	result = cs.HandleKey(keyRunes("d"), "@")
	handled = result.Consume
	assert.False(t, handled)
	assert.True(t, cs.PopupActive())
	assert.Equal(t, []string{"docs/"}, matchValues(cs.Matches()))
}

func TestPathCompletionWordBoundary(t *testing.T) {
	t.Parallel()

	cs := newTestCompletionState(t, nil, nil, func(string) []fs.PathMatch {
		return []fs.PathMatch{{Path: "docs/", IsDir: true}}
	})

	result := cs.HandleKey(keyRunes("@"), "user")
	handled := result.Consume
	assert.False(t, handled)
	assert.False(t, result.Consume)
	assert.False(t, cs.InPathCompletion())

	result = cs.HandleKey(keyRunes("@"), "see ")
	handled = result.Consume
	assert.False(t, handled)
	assert.True(t, cs.InPathCompletion())
}

func TestPathCompletionTabAndAccept(t *testing.T) {
	t.Parallel()

	pathFn := func(query string) []fs.PathMatch {
		switch query {
		case "do", "d":
			return []fs.PathMatch{{Path: "docs/", IsDir: true}}
		case "docs/":
			return []fs.PathMatch{
				{Path: "docs/api/", IsDir: true},
				{Path: "docs/readme.md", IsDir: false},
			}
		default:
			return nil
		}
	}
	cs := newTestCompletionState(t, nil, nil, pathFn)

	cs.HandleKey(keyRunes("@"), "")
	cs.HandleKey(keyRunes("d"), "@")
	cs.HandleKey(keyRunes("o"), "@d")
	require.True(t, cs.PopupActive())

	result := cs.HandleKey(tea.KeyMsg{Type: tea.KeyTab}, "@do")
	handled := result.Consume
	assert.True(t, handled)
	assert.True(t, result.Consume)
	assert.Equal(t, "@docs/", result.Replace)
	assert.Equal(t, input.CompletionLeaderPath, result.Leader)
	assert.True(t, cs.PopupActive())
	assert.Equal(t, []string{"docs/api/", "docs/readme.md"}, matchValues(cs.Matches()))

	cs.HandleKey(tea.KeyMsg{Type: tea.KeyDown}, "@docs/")
	assert.Equal(t, 1, cs.Selected())

	result = cs.HandleKey(tea.KeyMsg{Type: tea.KeyEnter}, "@docs/")
	handled = result.Consume
	assert.True(t, handled)
	assert.True(t, result.Consume)
	assert.Equal(t, "docs/readme.md", result.Replace)
	assert.Equal(t, input.CompletionLeaderPath, result.Leader)
	assert.False(t, cs.InPathCompletion())
}

func TestPathCompletionEscDismiss(t *testing.T) {
	t.Parallel()

	cs := newTestCompletionState(t, nil, nil, func(string) []fs.PathMatch {
		return []fs.PathMatch{{Path: "docs/", IsDir: true}}
	})
	cs.HandleKey(keyRunes("@"), "")
	cs.HandleKey(keyRunes("d"), "@")
	require.True(t, cs.PopupActive())

	result := cs.HandleKey(tea.KeyMsg{Type: tea.KeyEsc}, "@d")
	handled := result.Consume
	assert.True(t, handled)
	assert.True(t, result.Consume)
	assert.False(t, cs.InPathCompletion())
	assert.False(t, cs.PopupActive())
}

func TestVisibleRangeWindow(t *testing.T) {
	t.Parallel()

	matches := make([]fs.PathMatch, 0, 6)
	for _, name := range []string{"a", "b", "c", "d", "e", "f"} {
		matches = append(matches, fs.PathMatch{Path: name})
	}

	cs := newTestCompletionState(t, nil, nil, func(string) []fs.PathMatch { return matches })
	cs.HandleKey(keyRunes("@"), "")
	cs.HandleKey(keyRunes("x"), "@")
	require.True(t, cs.PopupActive())
	require.Len(t, cs.Matches(), 6)

	start, end := cs.VisibleRange()
	assert.Equal(t, 0, start)
	assert.Equal(t, 4, end)
	// 4 items + help + 2 border
	assert.Equal(t, 7, cs.PopupLineCount())

	for range 5 {
		cs.HandleKey(tea.KeyMsg{Type: tea.KeyDown}, "@x")
	}

	assert.Equal(t, 5, cs.Selected())
	start, end = cs.VisibleRange()
	assert.Equal(t, 2, start)
	assert.Equal(t, 6, end)
}

func TestCommandCompletionPopup(t *testing.T) {
	t.Parallel()

	commandFn := func(prefix string) []string {
		switch prefix {
		case "", "h", "he":
			return []string{"help", "history"}
		default:
			return nil
		}
	}
	cs := newTestCompletionState(t, commandFn, nil, nil)

	result := cs.HandleKey(keyRunes("/"), "")
	handled := result.Consume
	assert.False(t, handled)
	assert.True(t, cs.InCommandCompletion())
	assert.True(t, cs.PopupActive())
	assert.Equal(t, []string{"help", "history"}, matchValues(cs.Matches()))
	assert.Equal(t, []string{"help", "history"}, matchLabels(cs.Matches()))

	cs.HandleKey(keyRunes("h"), "/")
	result = cs.HandleKey(tea.KeyMsg{Type: tea.KeyEnter}, "/h")
	handled = result.Consume
	assert.True(t, handled)
	assert.Equal(t, "/help", result.Replace)
	assert.Equal(t, input.CompletionLeaderCommand, result.Leader)
	assert.False(t, cs.InCompletion())
}

func TestCommandCompletionUsageValue(t *testing.T) {
	t.Parallel()

	commandFn := func(string) []string {
		return []string{"mode <plan|work>", "help"}
	}
	cs := newTestCompletionState(t, commandFn, nil, nil)
	cs.HandleKey(keyRunes("/"), "")
	require.True(t, cs.PopupActive())
	assert.Equal(t, []string{"mode", "help"}, matchValues(cs.Matches()))
	assert.Equal(t, []string{"mode <plan|work>", "help"}, matchLabels(cs.Matches()))

	result := cs.HandleKey(tea.KeyMsg{Type: tea.KeyEnter}, "/")
	assert.Equal(t, "/mode", result.Replace)
}

func TestSkillCompletionAccept(t *testing.T) {
	t.Parallel()

	skillFn := func(prefix string) []*skills.Skill {
		if prefix == "" || prefix == "r" {
			return []*skills.Skill{
				{Name: "review", Description: "Review the code carefully"},
			}
		}

		return nil
	}
	cs := newTestCompletionState(t, nil, skillFn, nil)

	cs.HandleKey(keyRunes("$"), "")
	assert.True(t, cs.InSkillCompletion())
	assert.True(t, cs.PopupActive())
	assert.Equal(t, []string{"review"}, matchValues(cs.Matches()))

	result := cs.HandleKey(tea.KeyMsg{Type: tea.KeyEnter}, "$")
	assert.True(t, result.Consume)
	assert.Equal(t, "$review", result.Replace)
	assert.Equal(t, input.CompletionLeaderSkill, result.Leader)
}

func TestReplaceActiveToken(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		value       string
		replacement string
		leader      byte
		want        string
	}{
		{
			name:        "path_bare_token",
			value:       "@doc",
			replacement: "docs/",
			leader:      '@',
			want:        "docs/",
		},
		{
			name:        "path_with_prefix",
			value:       "look at @doc",
			replacement: "docs/readme.md",
			leader:      '@',
			want:        "look at docs/readme.md",
		},
		{
			name:        "path_segment_fill_keeps_at",
			value:       "x @do",
			replacement: "@docs/",
			leader:      '@',
			want:        "x @docs/",
		},
		{
			name:        "path_only_last_at_token",
			value:       "email@x @fi",
			replacement: "file.go",
			leader:      '@',
			want:        "email@x file.go",
		},
		{
			name:        "command_whole_line",
			value:       "/he",
			replacement: "/help",
			leader:      '/',
			want:        "/help",
		},
		{
			name:        "skill_with_prefix",
			value:       "use $re",
			replacement: "$review",
			leader:      '$',
			want:        "use $review",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			got := input.ReplaceActiveToken(test.value, test.replacement, test.leader)
			assert.Equal(t, test.want, got)
		})
	}
}
