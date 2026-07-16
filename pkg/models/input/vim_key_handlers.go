package input

import (
	"strings"

	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
)

const (
	simpleMoveKeys = "hjkl0$"
	insertKeys     = "iIaAoO"
	wordMoveKeys   = "wWeEbB"
)

type vimOperator uint8

const (
	vimOperatorNone vimOperator = iota
	vimOperatorDelete
)

type vimCommandState struct {
	operator    vimOperator
	prefixCount int
	motionCount int
}

type vimCommandKind uint8

const (
	vimCommandNone vimCommandKind = iota
	vimCommandDispatch
	vimCommandDeleteLines
	vimCommandDeleteBoundary
)

type vimCommand struct {
	kind  vimCommandKind
	key   string
	count int
}

func (s *vimCommandState) reset() {
	*s = vimCommandState{}
}

func (s *vimCommandState) active() bool {
	return s.operator != vimOperatorNone || s.prefixCount != 0 || s.motionCount != 0
}

func (s *vimCommandState) count() int {
	return max(s.prefixCount, 1) * max(s.motionCount, 1)
}

func (s *vimCommandState) accept(key rune) vimCommand {
	if s.operator != vimOperatorNone {
		if command, consumed := s.acceptOperatorKey(key); consumed {
			return command
		}
	}

	return s.acceptNormalKey(key)
}

func (s *vimCommandState) acceptOperatorKey(key rune) (vimCommand, bool) {
	if key >= '0' && key <= '9' && (key != '0' || s.motionCount != 0) {
		s.motionCount = s.motionCount*10 + int(key-'0')

		return vimCommand{}, true
	}

	switch key {
	case 'd':
		count := s.count()
		s.reset()

		return vimCommand{kind: vimCommandDeleteLines, count: count}, true
	case '0', '$':
		s.reset()

		return vimCommand{kind: vimCommandDeleteBoundary, key: string(key)}, true
	default:
		s.reset()

		return vimCommand{}, false
	}
}

func (s *vimCommandState) acceptNormalKey(key rune) vimCommand {
	if key >= '0' && key <= '9' && (key != '0' || s.prefixCount != 0) {
		s.prefixCount = s.prefixCount*10 + int(key-'0')

		return vimCommand{}
	}

	if key == 'd' {
		s.operator = vimOperatorDelete

		return vimCommand{}
	}

	s.prefixCount = 0

	return vimCommand{kind: vimCommandDispatch, key: string(key)}
}

func (m *Model) handleNormalModeVimKey(keys string) tea.Cmd {
	commands := make([]tea.Cmd, 0, len([]rune(keys)))

	for _, key := range keys {
		command := m.vimState.accept(key)

		switch command.kind {
		case vimCommandDispatch:
			commands = append(commands, m.dispatchNormalVimKey(command.key))
		case vimCommandDeleteLines:
			m.deleteLines(command.count)
		case vimCommandDeleteBoundary:
			m.deleteToLineBoundary(command.key)
		case vimCommandNone:
		}
	}

	return tea.Batch(commands...)
}

func (m *Model) dispatchNormalVimKey(key string) tea.Cmd {
	switch {
	case strings.Contains(simpleMoveKeys, key):
		return m.handleVimSimpleMove(key)
	case strings.Contains(insertKeys, key):
		return m.handleVimInsertKey(key)
	case strings.Contains(wordMoveKeys, key):
		return m.handleVimWordMove(key)
	case key == "p":
		return textarea.Paste
	default:
		return nil
	}
}

func (m *Model) handleVimSimpleMove(key string) tea.Cmd {
	if !strings.Contains(simpleMoveKeys, key) {
		return nil
	}

	if key == "j" || key == "k" {
		keyType := tea.KeyDown
		if key == "k" {
			keyType = tea.KeyUp
		}

		var cmd tea.Cmd

		m.textarea, cmd = m.textarea.Update(tea.KeyMsg{Type: keyType})

		return cmd
	}

	position := textareaLogicalPosition(m.textarea)
	lines := strings.Split(m.textarea.Value(), "\n")

	switch key {
	case "h":
		position.column--
	case "l":
		position.column++
	case "0":
		position.column = 0
	case "$":
		position.column = len([]rune(lines[position.line]))
	}

	position.column = min(max(position.column, 0), len([]rune(lines[position.line])))
	setLogicalCursor(&m.textarea, position)

	return nil
}

func (m *Model) handleVimInsertKey(key string) tea.Cmd {
	if !strings.Contains(insertKeys, key) {
		return nil
	}

	m.vimState.reset()
	m.setInputMode(modeInsert)
	m.textarea.Focus()
	m.statusline.SetFocus(true)

	content := m.textarea.Value()
	position := textareaLogicalPosition(m.textarea)
	start, end := currentLineBounds(content, position.line)

	switch key {
	case "i":
	case "I":
		position.column = 0
	case "a":
		position.column++
	case "A":
		position.column = end - start
	case "o":
		runes := []rune(content)
		runes = append(runes[:end], append([]rune{'\n'}, runes[end:]...)...)
		m.textarea.SetValue(string(runes))

		position = logicalPosition{line: position.line + 1}
	case "O":
		runes := []rune(content)
		runes = append(runes[:start], append([]rune{'\n'}, runes[start:]...)...)
		m.textarea.SetValue(string(runes))

		position.column = 0
	}

	setLogicalCursor(&m.textarea, position)

	return textarea.Blink
}

func (m *Model) handleVimWordMove(key string) tea.Cmd {
	if !strings.Contains(wordMoveKeys, key) {
		return nil
	}

	content := m.textarea.Value()
	position := textareaDocumentOffset(m.textarea)
	newPosition := position

	switch key {
	case "w":
		newPosition = findNextWord(content, position)
	case "W":
		newPosition = findNextWORD(content, position)
	case "e":
		newPosition = findEndOfWord(content, position)
	case "E":
		newPosition = findEndOfWORD(content, position)
	case "b":
		newPosition = findPrevWord(content, position)
	case "B":
		newPosition = findPrevWORD(content, position)
	}

	setDocumentCursor(&m.textarea, content, newPosition)

	return nil
}

func (m *Model) deleteToLineBoundary(key string) {
	content := []rune(m.textarea.Value())
	position := textareaLogicalPosition(m.textarea)
	start, end := currentLineBoundsRunes(content, position.line)

	cursor := min(start+position.column, end)
	if key == "0" {
		content = append(content[:start], content[cursor:]...)
	} else {
		content = append(content[:cursor], content[end:]...)
	}

	m.textarea.SetValue(string(content))

	if key == "0" {
		position.column = 0
	}

	setLogicalCursor(&m.textarea, position)
}

func (m *Model) deleteLines(count int) {
	content := []rune(m.textarea.Value())
	line := m.textarea.Line()
	lastDeletedLine := min(line+max(count, 1)-1, lineCountRunes(content)-1)
	start, _ := currentLineBoundsRunes(content, line)
	_, end := currentLineBoundsRunes(content, lastDeletedLine)

	if end < len(content) {
		end++
	} else if start > 0 {
		start--
	}

	content = append(content[:start], content[end:]...)
	m.textarea.SetValue(string(content))
	setLogicalCursor(&m.textarea, logicalPosition{line: min(line, m.textarea.LineCount()-1)})
}
