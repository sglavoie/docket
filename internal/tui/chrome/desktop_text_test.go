package chrome

import (
	"testing"

	"charm.land/bubbles/v2/textarea"
)

func TestDesktopTextareaRestoresMultilineUnicodeCaret(t *testing.T) {
	input := textarea.New()
	input.SetWidth(10)
	input.SetHeight(4)
	input.SetValue("a long wrapped first line\nsecond 🐈 line\nlast")
	input.MoveToEnd()
	input.SetCursorColumn(2)
	snapshot := DesktopTextarea(&input, nil)
	input.SetValue("changed")
	snapshot.Restore(snapshot.Text, snapshot.Row, snapshot.Column)
	restored := DesktopTextarea(&input, nil)
	if restored.Text != snapshot.Text || restored.Row != snapshot.Row || restored.Column != snapshot.Column {
		t.Fatalf("restored text/caret = %q (%d,%d), want %q (%d,%d)", restored.Text, restored.Row, restored.Column, snapshot.Text, snapshot.Row, snapshot.Column)
	}
}
