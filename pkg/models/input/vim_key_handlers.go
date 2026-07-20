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
	switch v.operator {
	case vimOperatorDelete:
		if command, consumed := v.acceptDeleteOperatorKey(key); consumed {
			return command
		}
	case vimOperatorNone:
		return v.acceptNormalKey(key)
	}

	return vimCommand{}
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

	cmd := vimCommand{kind: vimCommandDispatch, key: string(key), count: v.count()}
	v.prefixCount = 0

	return cmd
}

func (m *Model) handleNormalModeVimKey(keys string) tea.Cmd {
	commands := make([]tea.Cmd, 0, len([]rune(keys)))

	for _, key := range keys {
		command := m.vimState.accept(key)

		switch command.kind {
		case vimCommandDispatch:
			commands = append(commands, m.dispatchNormalVimKey(command))
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

func (m *Model) dispatchNormalVimKey(command vimCommand) tea.Cmd {
	key := command.key

	switch {
	case strings.Contains(simpleMoveKeys, key):
		return m.handleVimSimpleMove(key)
	case strings.Contains(insertKeys, key):
		return m.handleVimInsertKey(key)
	case strings.Contains(wordMoveKeys, key):
		return m.handleVimWordMove(command)
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

func (m *Model) handleVimWordMove(command vimCommand) tea.Cmd {
	if !strings.Contains(wordMoveKeys, command.key) {
		return nil
	}

	content := m.textarea.Value()
	position := textareaDocumentOffset(m.textarea)
	newPosition := jumpWordBoundaries(content, command.key, position, command.count)
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

func (m *Model) deleteWords(command vimCommand) {
	if !strings.Contains(wordMoveKeys, command.key) {
		return
	}

	// TODO: handle runes?
	content := m.textarea.Value()
	initialPosition := textareaDocumentOffset(m.textarea)
	jumpPosition := jumpWordBoundaries(content, command.key, initialPosition, command.count)

	// We want to remove the final character as well, which is where eE will land us
	if command.key == "e" || command.key == "E" {
		jumpPosition++
	}

	newPosition := initialPosition

	if jumpPosition < initialPosition {
		content = content[:jumpPosition] + content[initialPosition:]
		newPosition = jumpPosition
	} else if jumpPosition > initialPosition {
		content = content[:initialPosition] + content[jumpPosition:]
	}

	m.textarea.SetValue(content)

	setDocumentCursor(&m.textarea, content, newPosition)
}

func jumpWordBoundaries(content, key string, initialPosition, count int) int {
	jumpPosition := initialPosition

	for range count {
		switch key {
		case "w":
			jumpPosition = findNextWord(content, jumpPosition)
		case "W":
			jumpPosition = findNextWORD(content, jumpPosition)
		case "e":
			jumpPosition = findEndOfWord(content, jumpPosition)
		case "E":
			jumpPosition = findEndOfWORD(content, jumpPosition)
		case "b":
			jumpPosition = findPrevWord(content, jumpPosition)
		case "B":
			jumpPosition = findPrevWORD(content, jumpPosition)
		}
	}

	return jumpPosition
}
