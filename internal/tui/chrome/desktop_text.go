package chrome

import (
	"strings"

	"charm.land/bubbles/v2/textarea"
	"charm.land/bubbles/v2/textinput"
)

// DesktopTextBuffer exposes only the focused editable buffer. Restoring it
// changes a draft, never commits a task or edits a different field.
type DesktopTextBuffer struct {
	Key         any
	Text        string
	Row, Column int
	Restore     func(text string, row, column int)
}

type DesktopTextProvider interface{ DesktopTextBuffer() *DesktopTextBuffer }

func DesktopInput(input *textinput.Model, changed func()) *DesktopTextBuffer {
	return &DesktopTextBuffer{Key: input, Text: input.Value(), Column: input.Position(), Restore: func(text string, _, column int) {
		input.SetValue(text)
		input.SetCursor(column)
		if changed != nil {
			changed()
		}
	}}
}

func DesktopTextarea(input *textarea.Model, changed func()) *DesktopTextBuffer {
	return &DesktopTextBuffer{Key: input, Text: input.Value(), Row: input.Line(), Column: input.Column(), Restore: func(text string, row, column int) {
		input.SetValue(text)
		input.MoveToBegin()
		row = min(row, len(strings.Split(text, "\n"))-1)
		for input.Line() < row {
			input.CursorEnd()
			input.CursorDown()
		}
		input.SetCursorColumn(column)
		*input, _ = input.Update(nil)
		if changed != nil {
			changed()
		}
	}}
}
