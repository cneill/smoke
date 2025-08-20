package input

import (
	"log/slog"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
)

const (
	simpleMoveKeys = "hjkl0$"
	insertKeys     = "iIaAoO"
	wordMoveKeys   = "wWeEbB"
)

func (m *Model) handleNormalModeVimKey(key string) tea.Cmd {
	switch {
	case key == "d" || m.pendingD:
		return m.handleVimDelete(key)
	case strings.Contains(simpleMoveKeys, key):
		return m.handleVimSimpleMove(key)
	case strings.Contains(insertKeys, key):
		return m.handleVimInsertKey(key)
	case strings.Contains(wordMoveKeys, key):
		return m.handleVimWordMove(key)
	case key == "p":
		return textarea.Paste
	}

	return nil
}

func (m *Model) handleVimSimpleMove(key string) tea.Cmd {
	if !strings.Contains(simpleMoveKeys, key) {
		return nil
	}

	var sendKey tea.KeyType

	switch key {
	case "h":
		sendKey = tea.KeyLeft
	case "j":
		sendKey = tea.KeyDown
	case "k":
		sendKey = tea.KeyUp
	case "l":
		sendKey = tea.KeyRight
	case "0":
		m.textarea.CursorStart()
		return nil
	case "$":
		m.textarea.CursorEnd()
		return nil
	}

	m.textarea, _ = m.textarea.Update(tea.KeyMsg{Type: sendKey})

	return nil
}

func (m *Model) handleVimInsertKey(key string) tea.Cmd {
	if !strings.Contains(insertKeys, key) {
		return nil
	}

	m.setMode(modeInsert)
	m.textarea.Focus()

	switch key {
	case "i":
		// just enter insert mode where the cursor is
	case "I":
		m.textarea.CursorStart()
	case "a":
		m.textarea, _ = m.textarea.Update(tea.KeyMsg{Type: tea.KeyRight})
	case "A":
		m.textarea.CursorEnd()
	case "o":
		m.textarea.CursorEnd()
		m.textarea.InsertString("\n")
	case "O":
		m.textarea.CursorStart()
		m.textarea.InsertString("\n")
		m.textarea.CursorUp()
	}

	return textarea.Blink
}

func (m *Model) handleVimWordMove(key string) tea.Cmd {
	if !strings.Contains(wordMoveKeys, key) {
		return nil
	}

	var (
		content = m.textarea.Value()
		pos     = m.textarea.LineInfo().ColumnOffset
		newPos  int
	)

	switch key {
	case "w":
		// Move to beginning of next word
		newPos = findNextWord(content, pos)
	case "W":
		// Move to beginning of next WORD
		newPos = findNextWORD(content, pos)
	case "e":
		// Move to end of current/next word
		newPos = findEndOfWord(content, pos)
	case "E":
		// Move to end of current/next WORD
		newPos = findEndOfWORD(content, pos)
	case "b":
		// Move backward to beginning of word
		newPos = findPrevWord(content, pos)
	case "B":
		// Move backward to beginning of WORD
		newPos = findPrevWORD(content, pos)
	}

	m.textarea.SetCursor(newPos)

	return nil
}

func (m *Model) handleVimDelete(key string) tea.Cmd {
	var (
		content = m.textarea.Value()
		lines   = strings.Split(content, "\n")
		lineNum = m.textarea.Line()
		info    = m.textarea.LineInfo()
	)

	if key == "d" {
		slog.Debug("got d")

		if !m.pendingD {
			m.pendingD = true
			m.lastD = time.Now()
		} else {
			if time.Since(m.lastD) <= time.Second {
				// delete line
			}

			m.pendingD = false
			m.lastD = time.Time{}
		}

		return nil
	}

	if !m.pendingD {
		return nil
	}

	switch key {
	case "0":
		slog.Debug("got delete to beginning of line")

		newLines := []string{}
		if lineNum > 0 {
			newLines = append(newLines, lines[0:lineNum]...)
		}

		newLines = append(newLines, lines[lineNum][info.ColumnOffset:info.Width-1])

		if len(lines) > lineNum {
			newLines = append(newLines, lines[lineNum+1:]...)
		}

		m.textarea.SetValue(strings.Join(newLines, "\n"))

		m.pendingD = false
		m.lastD = time.Time{}
		// TODO: set cursor position
	case "$":
		slog.Debug("got delete to end of line")

		newLines := []string{}
		if lineNum > 0 {
			newLines = append(newLines, lines[0:lineNum]...)
		}

		newLines = append(newLines, lines[lineNum][0:info.ColumnOffset])

		if len(lines) > lineNum {
			newLines = append(newLines, lines[lineNum+1:]...)
		}

		m.textarea.SetValue(strings.Join(newLines, "\n"))

		m.pendingD = false
		m.lastD = time.Time{}
		// TODO: set cursor position
	}

	return nil
}
