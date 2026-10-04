package grammar

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// cursorByteOffset translates the textarea's logical line and rune column to
// the byte offsets used by the grammar, independently of visual wrapping.
func (q *QuickCaptureInput) cursorByteOffset() int {
	lines := strings.Split(q.Input.Value(), "\n")
	row := min(q.Input.Line(), len(lines)-1)
	offset := 0
	for _, line := range lines[:row] {
		offset += len(line) + 1
	}
	runes := []rune(lines[row])
	return offset + len(string(runes[:min(q.Input.Column(), len(runes))]))
}

// captureTokenEnd includes the untyped remainder when editing inside a token.
func (q *QuickCaptureInput) captureTokenEnd() int {
	value := q.Input.Value()
	end := q.cursorByteOffset()
	for _, r := range value[end:] {
		if unicode.IsSpace(r) {
			break
		}
		end += utf8.RuneLen(r)
	}
	return end
}

func (q *QuickCaptureInput) setCursorByteOffset(offset int) {
	prefix := q.Input.Value()[:offset]
	row := strings.Count(prefix, "\n")
	column := utf8.RuneCountInString(prefix[strings.LastIndexByte(prefix, '\n')+1:])
	q.Input.MoveToBegin()
	for q.Input.Line() < row {
		// Starting at the logical line's end skips its visual wraps.
		q.Input.CursorEnd()
		q.Input.CursorDown()
	}
	q.Input.SetCursorColumn(column)
	// SetCursorColumn alone does not reposition the textarea's viewport.
	q.Input, _ = q.Input.Update(nil)
}
