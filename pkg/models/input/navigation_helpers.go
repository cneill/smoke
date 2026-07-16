package input

import (
	"strings"

	"github.com/charmbracelet/bubbles/textarea"
)

type logicalPosition struct {
	line   int
	column int
}

func textareaLogicalPosition(model textarea.Model) logicalPosition {
	info := model.LineInfo()

	return logicalPosition{
		line:   model.Line(),
		column: info.StartColumn + info.CharOffset,
	}
}

func textareaDocumentOffset(model textarea.Model) int {
	position := textareaLogicalPosition(model)

	return documentCursorOffset(model.Value(), position.line, position.column)
}

func documentCursorOffset(content string, line, column int) int {
	return documentCursorOffsetRunes([]rune(content), line, column)
}

func documentCursorOffsetRunes(content []rune, line, column int) int {
	start, end := currentLineBoundsRunes(content, line)

	return min(start+max(column, 0), end)
}

func documentPosition(content string, offset int) logicalPosition {
	runes := []rune(content)
	offset = min(max(offset, 0), len(runes))
	position := logicalPosition{}

	for _, r := range runes[:offset] {
		if r == '\n' {
			position.line++
			position.column = 0

			continue
		}

		position.column++
	}

	return position
}

func lineEnd(content []rune, start int) int {
	for i := min(max(start, 0), len(content)); i < len(content); i++ {
		if content[i] == '\n' {
			return i
		}
	}

	return len(content)
}

func currentLineBounds(content string, line int) (int, int) {
	return currentLineBoundsRunes([]rune(content), line)
}

func currentLineBoundsRunes(content []rune, line int) (int, int) {
	line = max(line, 0)

	start := 0
	for range line {
		start = lineEnd(content, start)
		if start >= len(content) {
			return len(content), len(content)
		}

		start++
	}

	return start, lineEnd(content, start)
}

func setLogicalCursor(model *textarea.Model, position logicalPosition) {
	lineCount := model.LineCount()
	if lineCount == 0 {
		return
	}

	position.line = min(max(position.line, 0), lineCount-1)
	maxSteps := len([]rune(model.Value())) + lineCount + 1

	for steps := 0; model.Line() != position.line && steps < maxSteps; steps++ {
		if model.Line() < position.line {
			model.CursorDown()
		} else {
			model.CursorUp()
		}
	}

	if model.Line() == position.line {
		model.SetCursor(max(position.column, 0))
	}
}

func setDocumentCursor(model *textarea.Model, content string, offset int) {
	setLogicalCursor(model, documentPosition(content, offset))
}

func setLastRenderedRowStart(model *textarea.Model) {
	content := model.Value()
	setDocumentCursor(model, content, len([]rune(content)))
	model.SetCursor(model.LineInfo().StartColumn)
}

func lineCountRunes(content []rune) int {
	return strings.Count(string(content), "\n") + 1
}
