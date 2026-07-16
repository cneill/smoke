package input //nolint:testpackage

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/cneill/smoke/pkg/fs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newNavigationModel(t *testing.T, width ...int) *Model {
	t.Helper()

	modelWidth := 80
	if len(width) > 0 {
		modelWidth = width[0]
	}

	model, err := New(&Opts{
		Width:            modelWidth,
		Height:           5,
		CommandCompleter: func(string) []string { return nil },
		SkillCompleter:   func(string) []string { return nil },
		PathCompleter:    func(string) []fs.PathMatch { return nil },
	})
	require.NoError(t, err)

	return model
}

func keyRunes(s string) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
}

func setNormalInput(model *Model, content string, position logicalPosition) {
	model.textarea.SetValue(content)

	for model.textarea.Line() > position.line {
		model.textarea.CursorUp()
	}

	for model.textarea.Line() < position.line {
		model.textarea.CursorDown()
	}

	model.textarea.SetCursor(position.column)
	model.setInputMode(modeNormal)
}

func sendRuneKeys(m *Model, keys string) {
	for _, key := range keys {
		m.Update(keyRunes(string(key)))
	}
}

func assertCursor(t *testing.T, m *Model, want logicalPosition) {
	t.Helper()
	assert.Equal(t, want, textareaLogicalPosition(m.textarea))
}

func TestVimWordMotionsUseCurrentLogicalLine(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		key   string
		start logicalPosition
		want  logicalPosition
	}{
		{name: "next word", key: "w", start: logicalPosition{line: 1}, want: logicalPosition{line: 1, column: 5}},
		{name: "next WORD", key: "W", start: logicalPosition{line: 1}, want: logicalPosition{line: 1, column: 11}},
		{name: "end word", key: "e", start: logicalPosition{line: 1}, want: logicalPosition{line: 1, column: 4}},
		{name: "end WORD", key: "E", start: logicalPosition{line: 1}, want: logicalPosition{line: 1, column: 9}},
		{name: "previous word", key: "b", start: logicalPosition{line: 1, column: 16}, want: logicalPosition{line: 1, column: 11}},
		{name: "previous WORD", key: "B", start: logicalPosition{line: 1, column: 16}, want: logicalPosition{line: 1, column: 11}},
		{name: "next word crosses newline", key: "w", start: logicalPosition{line: 1, column: 11}, want: logicalPosition{line: 2}},
		{name: "next WORD crosses newline", key: "W", start: logicalPosition{line: 1, column: 11}, want: logicalPosition{line: 2}},
		{name: "end word crosses newline", key: "e", start: logicalPosition{line: 1, column: 15}, want: logicalPosition{line: 2, column: 3}},
		{name: "end WORD crosses newline", key: "E", start: logicalPosition{line: 1, column: 15}, want: logicalPosition{line: 2, column: 3}},
		{name: "previous word crosses newline", key: "b", start: logicalPosition{line: 1}, want: logicalPosition{line: 0, column: 4}},
		{name: "previous WORD crosses newline", key: "B", start: logicalPosition{line: 1}, want: logicalPosition{line: 0}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			m := newNavigationModel(t)
			setNormalInput(m, "top.words\nalpha.beta gamma\nlast", tt.start)

			m.Update(keyRunes(tt.key))

			assertCursor(t, m, tt.want)
		})
	}
}

func TestVimWordMotionsHaveDistinctObservableTargets(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		key  string
		want string
	}{
		{name: "word stops at punctuation", key: "w", want: "alphaX.beta gamma"},
		{name: "WORD stops at whitespace", key: "W", want: "alpha.beta Xgamma"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			model := newNavigationModel(t)
			setNormalInput(model, "alpha.beta gamma", logicalPosition{})

			model.Update(keyRunes(tt.key))
			model.Update(keyRunes("i"))
			model.Update(keyRunes("X"))

			assert.Equal(t, tt.want, model.textarea.Value())
		})
	}
}

func TestVimWordMotionHandlesSoftWrapping(t *testing.T) {
	t.Parallel()

	model := newNavigationModel(t, 12)
	content := "first\nxx alpha beta gamma\nlast"
	setNormalInput(model, content, logicalPosition{line: 1, column: 15})

	model.Update(keyRunes("b"))

	assertCursor(t, model, logicalPosition{line: 1, column: 14})
	model.Update(keyRunes("b"))
	assertCursor(t, model, logicalPosition{line: 1, column: 9})
	model.Update(keyRunes("b"))
	assertCursor(t, model, logicalPosition{line: 1, column: 3})

	model.Update(keyRunes("i"))
	model.Update(keyRunes("X"))
	assert.Equal(t, "first\nxx Xalpha beta gamma\nlast", model.textarea.Value())
}

func TestVimLineBoundaryMotionsAreLogicalLineLocal(t *testing.T) {
	t.Parallel()

	tests := []struct {
		key  string
		want logicalPosition
	}{
		{key: "0", want: logicalPosition{line: 1}},
		{key: "$", want: logicalPosition{line: 1, column: 10}},
	}

	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			t.Parallel()
			m := newNavigationModel(t, 10)
			setNormalInput(m, "short\nalpha beta\nthe last line", logicalPosition{line: 1, column: 7})

			m.Update(keyRunes(tt.key))

			assertCursor(t, m, tt.want)
		})
	}
}

func TestVimJKMatchesNativeArrowNavigation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		content  string
		position logicalPosition
		vimKeys  string
		arrows   []tea.KeyType
	}{
		{
			name: "down within soft-wrapped line", content: "first\nalpha beta gamma delta\nlast",
			position: logicalPosition{line: 1, column: 3}, vimKeys: "j", arrows: []tea.KeyType{tea.KeyDown},
		},
		{
			name: "up within soft-wrapped line", content: "first\nalpha beta gamma delta\nlast",
			position: logicalPosition{line: 1, column: 17}, vimKeys: "k", arrows: []tea.KeyType{tea.KeyUp},
		},
		{
			name: "down through wrapped and explicit rows", content: "first\nalpha beta gamma delta\nlast line",
			position: logicalPosition{line: 1, column: 3}, vimKeys: "jjj",
			arrows: []tea.KeyType{tea.KeyDown, tea.KeyDown, tea.KeyDown},
		},
		{
			name: "up through wrapped and explicit rows", content: "first line\nalpha beta gamma delta\nlast",
			position: logicalPosition{line: 1, column: 17}, vimKeys: "kkk",
			arrows: []tea.KeyType{tea.KeyUp, tea.KeyUp, tea.KeyUp},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			vimModel := newNavigationModel(t, 12)
			setNormalInput(vimModel, tt.content, tt.position)

			control := newNavigationModel(t, 12)
			control.textarea.SetValue(tt.content)

			for control.textarea.Line() > tt.position.line {
				control.textarea.CursorUp()
			}

			control.textarea.SetCursor(tt.position.column)

			sendRuneKeys(vimModel, tt.vimKeys)

			for _, arrow := range tt.arrows {
				control.Update(tea.KeyMsg{Type: arrow})
			}

			vimModel.Update(keyRunes("i"))
			vimModel.Update(keyRunes("X"))
			control.Update(keyRunes("X"))

			assert.Equal(t, control.textarea.Value(), vimModel.textarea.Value())
		})
	}
}

func TestVimInsertCommandsUseCurrentLogicalLine(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		key     string
		want    string
		cursor  logicalPosition
		insert  string
		updated string
	}{
		{
			name:    "insert at line start",
			key:     "I",
			want:    "one\nalpha beta\ntail",
			cursor:  logicalPosition{line: 1},
			insert:  "X",
			updated: "one\nXalpha beta\ntail",
		},
		{
			name:    "append at line end",
			key:     "A",
			want:    "one\nalpha beta\ntail",
			cursor:  logicalPosition{line: 1, column: 10},
			insert:  "X",
			updated: "one\nalpha betaX\ntail",
		},
		{
			name:    "open below",
			key:     "o",
			want:    "one\nalpha beta\n\ntail",
			cursor:  logicalPosition{line: 2},
			insert:  "X",
			updated: "one\nalpha beta\nX\ntail",
		},
		{
			name:    "open above",
			key:     "O",
			want:    "one\n\nalpha beta\ntail",
			cursor:  logicalPosition{line: 1},
			insert:  "X",
			updated: "one\nX\nalpha beta\ntail",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			model := newNavigationModel(t, 10)
			setNormalInput(model, "one\nalpha beta\ntail", logicalPosition{line: 1, column: 6})

			model.Update(keyRunes(tt.key))

			assert.Equal(t, modeInsert, model.mode)
			assert.Equal(t, tt.want, model.textarea.Value())
			assertCursor(t, model, tt.cursor)

			model.Update(keyRunes(tt.insert))
			assert.Equal(t, tt.updated, model.textarea.Value())
		})
	}
}

func TestHistoryNavigationDispatchesAtLogicalBoundaries(t *testing.T) {
	t.Parallel()

	model := newNavigationModel(t)
	model.userHistory = []string{"old one\nold two", "new one\nnew two"}

	model.Update(tea.KeyMsg{Type: tea.KeyUp})
	assert.Equal(t, "new one\nnew two", model.textarea.Value())
	assertCursor(t, model, logicalPosition{line: 1})

	model.Update(tea.KeyMsg{Type: tea.KeyUp})
	assert.Equal(t, "new one\nnew two", model.textarea.Value())
	assertCursor(t, model, logicalPosition{})

	model.Update(tea.KeyMsg{Type: tea.KeyUp})
	assert.Equal(t, "old one\nold two", model.textarea.Value())
	assertCursor(t, model, logicalPosition{line: 1})

	model.Update(tea.KeyMsg{Type: tea.KeyDown})
	assert.Equal(t, "new one\nnew two", model.textarea.Value())
	assertCursor(t, model, logicalPosition{})

	model.Update(tea.KeyMsg{Type: tea.KeyDown})
	assert.Equal(t, "new one\nnew two", model.textarea.Value())
	assertCursor(t, model, logicalPosition{line: 1})

	model.Update(tea.KeyMsg{Type: tea.KeyDown})
	assert.Empty(t, model.textarea.Value())
	assert.Nil(t, model.userHistoryIndex)
}

func TestHistoryNavigationRespectsSoftWrappedRows(t *testing.T) {
	t.Parallel()

	model := newNavigationModel(t, 12)
	model.userHistory = []string{"old", "alpha beta gamma delta"}

	model.Update(tea.KeyMsg{Type: tea.KeyUp})
	info := model.textarea.LineInfo()
	require.Greater(t, info.Height, 1)
	assert.Equal(t, info.Height-1, info.RowOffset)
	assert.Equal(t, 1, *model.userHistoryIndex)

	for range info.Height - 1 {
		model.Update(tea.KeyMsg{Type: tea.KeyUp})
		assert.Equal(t, "alpha beta gamma delta", model.textarea.Value())
		assert.Equal(t, 1, *model.userHistoryIndex)
	}

	assert.Zero(t, model.textarea.LineInfo().RowOffset)
	model.Update(tea.KeyMsg{Type: tea.KeyUp})
	assert.Equal(t, "old", model.textarea.Value())
	assert.Equal(t, 0, *model.userHistoryIndex)
}

func TestHistoryDoesNotStealNavigationFromEditedInput(t *testing.T) {
	t.Parallel()

	model := newNavigationModel(t)
	model.userHistory = []string{"history"}
	model.textarea.SetValue("draft\nmessage")
	setLogicalCursor(&model.textarea, logicalPosition{line: 1, column: 2})

	model.Update(tea.KeyMsg{Type: tea.KeyUp})

	assert.Equal(t, "draft\nmessage", model.textarea.Value())
	assert.Nil(t, model.userHistoryIndex)
	assertCursor(t, model, logicalPosition{column: 2})
}

func TestVimDeleteRanges(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		content  string
		position logicalPosition
		keys     string
		want     string
		cursor   logicalPosition
	}{
		{
			name: "to line end", content: "one\nalpha beta\ntail",
			position: logicalPosition{line: 1, column: 6}, keys: "d$",
			want: "one\nalpha \ntail", cursor: logicalPosition{line: 1, column: 6},
		},
		{
			name: "to line start", content: "one\nalpha beta\ntail",
			position: logicalPosition{line: 1, column: 6}, keys: "d0",
			want: "one\nbeta\ntail", cursor: logicalPosition{line: 1},
		},
		{
			name: "middle line", content: "one\ntwo\nthree",
			position: logicalPosition{line: 1, column: 2}, keys: "dd",
			want: "one\nthree", cursor: logicalPosition{line: 1},
		},
		{
			name: "count after operator", content: "one\ntwo\nthree\nfour",
			position: logicalPosition{line: 1}, keys: "d2d", want: "one\nfour", cursor: logicalPosition{line: 1},
		},
		{
			name: "count after operator clamps", content: "one\ntwo\nthree\nfour",
			position: logicalPosition{line: 1}, keys: "d5d", want: "one", cursor: logicalPosition{},
		},
		{
			name: "count before operator", content: "one\ntwo\nthree\nfour",
			position: logicalPosition{}, keys: "3dd", want: "four", cursor: logicalPosition{},
		},
		{
			name: "last line", content: "one\ntwo\nthree", position: logicalPosition{line: 2},
			keys: "dd", want: "one\ntwo", cursor: logicalPosition{line: 1},
		},
		{
			name: "all lines", content: "one\ntwo", position: logicalPosition{},
			keys: "5dd", want: "", cursor: logicalPosition{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			model := newNavigationModel(t, 12)
			setNormalInput(model, tt.content, tt.position)

			sendRuneKeys(model, tt.keys)

			assert.Equal(t, tt.want, model.textarea.Value())
			assertCursor(t, model, tt.cursor)
			assert.False(t, model.vimState.active())
		})
	}
}

func TestVimGroupedRuneCommands(t *testing.T) {
	t.Parallel()

	model := newNavigationModel(t)
	setNormalInput(model, "one\ntwo\nthree\nfour", logicalPosition{line: 1, column: 2})

	model.Update(keyRunes("d2d"))

	assert.Equal(t, "one\nfour", model.textarea.Value())
	assert.False(t, model.vimState.active())

	model.Update(keyRunes("i"))
	model.Update(keyRunes("X"))
	assert.Equal(t, "one\nXfour", model.textarea.Value())
}

func TestVimCommandStateTransitions(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		keys  string
		kind  vimCommandKind
		key   string
		count int
	}{
		{name: "delete current line", keys: "dd", kind: vimCommandDeleteLines, count: 1},
		{name: "count after operator", keys: "d2d", kind: vimCommandDeleteLines, count: 2},
		{name: "count before operator", keys: "3dd", kind: vimCommandDeleteLines, count: 3},
		{name: "multiplied counts", keys: "2d3d", kind: vimCommandDeleteLines, count: 6},
		{name: "delete to start", keys: "d0", kind: vimCommandDeleteBoundary, key: "0"},
		{name: "delete to end", keys: "d$", kind: vimCommandDeleteBoundary, key: "$"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			state := vimCommandState{}
			command := vimCommand{}

			for _, key := range tt.keys {
				if next := state.accept(key); next.kind != vimCommandNone {
					command = next
				}
			}

			assert.Equal(t, tt.kind, command.kind)
			assert.Equal(t, tt.key, command.key)
			assert.Equal(t, tt.count, command.count)
			assert.False(t, state.active())
		})
	}
}

func TestVimOperatorStateCancelsCleanly(t *testing.T) {
	t.Parallel()

	model := newNavigationModel(t)
	setNormalInput(model, "alpha beta", logicalPosition{})

	sendRuneKeys(model, "dxw")
	assertCursor(t, model, logicalPosition{column: 6})
	assert.False(t, model.vimState.active())

	model.Update(keyRunes("d"))
	assert.True(t, model.vimState.active())
	model.Update(tea.KeyMsg{Type: tea.KeyEsc})
	assert.False(t, model.vimState.active())
	assert.True(t, model.Focused())
	assert.Equal(t, modeNormal, model.mode)
}
