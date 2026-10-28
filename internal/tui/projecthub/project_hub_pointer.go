package projecthub

import (
	"fmt"

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
// key, and so owns the pointer too: an event inside the cells the last frame
// drew it on reaches the sheet's own answer, translated into those cells, and
// one outside it lands on nothing, never on the list beneath. A click also
// needs the sheet drawn whole, as a shell popup's does; a wheel step reads no
// row.
func (m *ProjectHubModel) HandlePointer(event route.PointerEvent) tea.Cmd {
	if m == nil || m.surface == nil {
		return nil
	}
	if m.actions != nil {
		placement := m.actionsPlacement
		if !placement.Contains(event.X, event.Y) || (!placement.Whole && !event.IsWheel()) {
			return nil
		}
		return m.actions.HandlePointer(event.Offset(placement.Col, placement.Row))
	}
	if m.pointerTop < 0 {
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

// PointerTarget is what a key the pointer pressed would act on: the action the
// sheet's cursor names while one stands, qualified by that sheet so a sheet
// reopened for another project is another target, and otherwise the project
// the list's cursor names.
func (m *ProjectHubModel) PointerTarget() string {
	if m == nil {
		return ""
	}
	if m.actions != nil {
		return fmt.Sprintf("sheet %p %s", m.actions, m.actions.PointerTarget())
	}
	return m.surface.PointerTarget()
}

var (
	_ route.PointerView       = (*ProjectHubModel)(nil)
	_ route.PointerTargetView = (*ProjectHubModel)(nil)
)
