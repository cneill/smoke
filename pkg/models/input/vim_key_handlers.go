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
	vimCommandDeleteWords
)

type vimCommand struct {
	kind  vimCommandKind
	key   string
	count int
}

func (v *vimCommandState) reset() {
	*v = vimCommandState{}
}

func (v *vimCommandState) active() bool {
	return v.operator != vimOperatorNone || v.prefixCount != 0 || v.motionCount != 0
}

func (v *vimCommandState) count() int {
	return max(v.prefixCount, 1) * max(v.motionCount, 1)
}

func (v *vimCommandState) accept(key rune) vimCommand {
	if v.operator != vimOperatorNone {
		if command, consumed := v.acceptOperatorKey(key); consumed {
			return command
		}
	}

	return v.acceptNormalKey(key)
}

func (v *vimCommandState) acceptOperatorKey(key rune) (vimCommand, bool) {
	switch v.operator {
	case vimOperatorDelete:
		return v.acceptDeleteOperatorKey(key)
	case vimOperatorNone:
		// Should not happen...
		return vimCommand{}, false
	}

	return vimCommand{}, false
}

func (v *vimCommandState) acceptDeleteOperatorKey(key rune) (vimCommand, bool) {
	if key >= '0' && key <= '9' && (key != '0' || v.motionCount != 0) {
		v.motionCount = v.motionCount*10 + int(key-'0')

		return vimCommand{}, true
	}

	switch {
	case key == 'd':
		count := v.count()
		v.reset()

		return vimCommand{kind: vimCommandDeleteLines, count: count}, true
	case key == '0' || key == '$':
		v.reset()

		return vimCommand{kind: vimCommandDeleteBoundary, key: string(key)}, true
	case strings.Contains(wordMoveKeys, string(key)):
		count := v.count()
		v.reset()

		return vimCommand{kind: vimCommandDeleteWords, count: count, key: string(key)}, true
	default:
		v.reset()

		return vimCommand{}, false
	}
}

func (v *vimCommandState) acceptNormalKey(key rune) vimCommand {
	if key >= '0' && key <= '9' && (key != '0' || v.prefixCount != 0) {
		v.prefixCount = v.prefixCount*10 + int(key-'0')

		return vimCommand{}
	}

	if key == 'd' {
		v.operator = vimOperatorDelete

		return vimCommand{}
	}

	v.prefixCount = 0

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
		case vimCommandDeleteWords:
			m.deleteWords(command)
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

// TODO: combine with move?
func (m *Model) deleteWords(command vimCommand) {
	if !strings.Contains(wordMoveKeys, command.key) {
		return
	}

	// TODO: handle runes?
	content := m.textarea.Value()
	initialPosition := textareaDocumentOffset(m.textarea)
	boundaryPosition := initialPosition

	for range command.count {
		switch command.key {
		case "w":
			boundaryPosition = findNextWord(content, boundaryPosition)
		case "W":
			boundaryPosition = findNextWORD(content, boundaryPosition)
		case "e":
			boundaryPosition = findEndOfWord(content, boundaryPosition) + 1 // remove the final character as well
		case "E":
			boundaryPosition = findEndOfWORD(content, boundaryPosition) + 1 // remove the final character as well
		case "b":
			boundaryPosition = findPrevWord(content, boundaryPosition)
		case "B":
			boundaryPosition = findPrevWORD(content, boundaryPosition)
		}
	}

	newPosition := initialPosition

	if boundaryPosition < initialPosition {
		content = content[:boundaryPosition] + content[initialPosition:]
		newPosition = boundaryPosition
	} else if boundaryPosition > initialPosition {
		content = content[:initialPosition] + content[boundaryPosition:]
	}

	m.textarea.SetValue(content)

	setDocumentCursor(&m.textarea, content, newPosition)
}
