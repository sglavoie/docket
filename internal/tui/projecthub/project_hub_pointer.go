package projecthub

import (
	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/docket/internal/tui/route"
)

// HandlePointer gives Select Project the list surface's pointer answer. The
// Hub is drawn from the screen's top-left cell, so the event's rows are the
// frame's and only the header above the list is subtracted. A click on a
// project seats the cursor there, a double-click is Enter on it, and a
// right-click is Space, the project's action sheet. The wheel steps the cursor
// a row, except over the preview pane, where it pages the project's tasks as
// Ctrl+F and Ctrl+B do. A click on the preview, the header, or the search field
// lands on nothing.
//
// While the query is being typed, Space is a character of it, so a right-click
// only seats. The action sheet the Hub draws in place of its list owns every
// key, and so swallows the pointer.
func (m *ProjectHubModel) HandlePointer(event route.PointerEvent) tea.Cmd {
	if m == nil || m.surface == nil || m.actions != nil || m.pointerTop < 0 {
		return nil
	}
	// A pointer event is a keystroke, so it abandons a half-typed gg.
	m.pendingTopChord = false
	pane := event.Offset(0, m.pointerTop)
	if event.IsWheel() && m.surface.PointerOverPreview(pane.X) {
		if event.WheelDelta > 0 {
			return route.PressKey("ctrl+f")
		}
		return route.PressKey("ctrl+b")
	}
	key := m.surface.AnswerPointer(pane, true)
	if m.HasActiveSearch() {
		if key == "space" {
			return nil
		}
	} else {
		// Every browse key retires the notice first, and the pointer is one.
		m.notice = ""
	}
	return route.PressKey(key)
}

var _ route.PointerView = (*ProjectHubModel)(nil)
