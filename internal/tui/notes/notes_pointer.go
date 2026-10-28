package notes

import (
	"fmt"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/docket/internal/tui/listsurface"
	"github.com/sglavoie/docket/internal/tui/route"
)

// notesPointerFrame is what the last frame drew: the screen row the list pane
// starts on, which note each of its rows was, and the first column of the
// preview pane, zero when the layout drew the list alone.
type notesPointerFrame struct {
	top         int
	rows        listsurface.ListPointerFrame
	previewLeft int
}

// HandlePointer gives Notes the list surface's pointer answer. Notes is drawn
// from the screen's top-left cell, so only the header above the list is
// subtracted. A click on a note seats the cursor there as j/k would, a
// double-click is Enter on it, and a right-click is Space, its menu. The wheel
// steps the cursor a row, except over the preview pane, where it pages the note
// as Ctrl+F and Ctrl+B do; a click on the preview body lands on nothing.
//
// The location picker, the rename input, and the two confirmations own every
// key, so they swallow the pointer. While the search is being typed, Enter and
// Space are its own keys, so a click only seats and the wheel only steps.
func (m *NotesViewModel) HandlePointer(event route.PointerEvent) tea.Cmd {
	if m == nil || m.LocationPicker != nil || m.renaming || m.confirmDiscard || m.Confirm.IsActive() {
		return nil
	}
	// A pointer event is a keystroke, so it abandons a half-typed gg.
	m.pendingTopChord = false
	overPreview := m.pointer.previewLeft > 0 && event.X >= m.pointer.previewLeft
	if event.IsWheel() {
		switch {
		case overPreview && event.WheelDelta > 0:
			return route.PressKey("ctrl+f")
		case overPreview:
			return route.PressKey("ctrl+b")
		}
		return route.PressKey(listsurface.ListPointerKey(event, false, true))
	}
	index, ok := m.pointer.rows.IndexAt(event.X, event.Y-m.pointer.top)
	if !ok || index >= len(m.VisibleRows) {
		return nil
	}
	if !m.searchEditing {
		// Every browse key clears the status line first; the click is one.
		m.ClearStatus()
	}
	m.Cursor = index
	key := listsurface.ListPointerKey(event, true, true)
	if m.searchEditing && (key == "enter" || key == "space") {
		return nil
	}
	return route.PressKey(key)
}

// PointerTarget is the note the cursor names, by location and filename.
func (m *NotesViewModel) PointerTarget() string {
	if m == nil || m.Cursor < 0 || m.Cursor >= len(m.VisibleRows) {
		return ""
	}
	return fmt.Sprintf("%+v", m.VisibleRows[m.Cursor].Identity())
}

var (
	_ route.PointerView       = (*NotesViewModel)(nil)
	_ route.PointerTargetView = (*NotesViewModel)(nil)
)
