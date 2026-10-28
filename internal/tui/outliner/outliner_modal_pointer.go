package outliner

import (
	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/docket/internal/tui/chrome"
	"github.com/sglavoie/docket/internal/tui/listsurface"
	"github.com/sglavoie/docket/internal/tui/route"
)

// The view-local modals answer the pointer ahead of the rows they stand on, in
// the order handleKeyMsg gives them keys: the relation picker, then an open
// menu. Each reads where the last render composited it, recorded below the way
// LastViewport is, so a hit is the cell the user saw and never one a second
// placement would compute. The host beneath a modal receives nothing.

// outlinerModalFrame is where the last render drew the bar and the modal over
// the host, in the Outliner's own cells.
type outlinerModalFrame struct {
	// menuLabels are the bar's drawn labels on the view's first row, empty when
	// no bar was drawn.
	menuLabels []menuLabelBox
	// dropdown is where the open menu was composited, and dropdownSurface the
	// list inside it whose rows the render recorded.
	dropdown        chrome.OverlayPlacement
	dropdownSurface *listsurface.ListSurfaceModel
	// picker is where the relation picker was composited, and pickerListTop the
	// popup row its list starts on.
	picker        chrome.OverlayPlacement
	pickerListTop int
}

// handleModalPointer answers an event a modal or the bar owns, and reports
// whether it did. Every event reaching an open modal is owned by it, whether or
// not it acts; with no modal open, only a click on a bar label is.
func (m *OutlinerModel) handleModalPointer(event route.PointerEvent) (tea.Cmd, bool) {
	switch {
	case m.RelationPickerOpen():
		return m.relationPickerPointer(event), true
	case m.MenuBar.Open:
		return m.openMenuPointer(event), true
	case event.IsWheel() || m.SimplifiedInlineControlOwnsInput():
		return nil, false
	}
	index, ok := m.menuLabelAt(event.X, event.Y)
	if !ok {
		return nil, false
	}
	if event.Button == route.PointerButtonLeft {
		m.OpenMenuBar()
		m.MenuBar.MenuIndex = index
	}
	return nil, true
}

// menuLabelAt reports which drawn bar label covers cell (x, y). The bar is the
// view's first row whenever one was drawn.
func (m *OutlinerModel) menuLabelAt(x, y int) (int, bool) {
	if y != 0 {
		return 0, false
	}
	for index, box := range m.modalFrame.menuLabels {
		if x >= box.StartCol && x < box.StartCol+box.Width {
			return index, true
		}
	}
	return 0, false
}

// openMenuPointer is the open menu's answer. A click on a label opens that
// menu, or closes the one already open under it, as its key does; a click on
// an enabled entry is Enter on that entry; a click on a disabled entry, a
// separator, or the frame does nothing; and a click anywhere else closes the
// menu, as Esc would, and does nothing more. The wheel over the dropdown walks
// the highlight as j/k do.
func (m *OutlinerModel) openMenuPointer(event route.PointerEvent) tea.Cmd {
	menus, ok := m.OpenMenuBarMenus()
	if !ok {
		return nil
	}
	frame := m.modalFrame
	inside := frame.dropdownSurface != nil && frame.dropdown.Contains(event.X, event.Y)
	if event.IsWheel() {
		if inside {
			entries := menus[m.MenuBar.MenuIndex].Entries
			m.MenuBar.selectItem(StepMenuBarItem(entries, m.MenuBar.ItemIndex, event.WheelDelta))
		}
		return nil
	}
	if index, onLabel := m.menuLabelAt(event.X, event.Y); onLabel {
		if index == m.MenuBar.MenuIndex {
			m.closeMenuBar()
		} else if index < len(menus) {
			m.MenuBar.MenuIndex, m.MenuBar.ItemIndex = index, 0
		}
		return nil
	}
	if !inside {
		m.closeMenuBar()
		return nil
	}
	if event.Button != route.PointerButtonLeft || !frame.dropdown.Whole {
		return nil
	}
	local := event.Offset(frame.dropdown.Col+chrome.OverlayBoxContentLeft, frame.dropdown.Row+chrome.OverlayBoxUntitledContentTop)
	id, hit := frame.dropdownSurface.PointerItemAt(local.X, local.Y)
	if !hit {
		return nil
	}
	entries := menus[m.MenuBar.MenuIndex].Entries
	index, ok := menuBarDropdownItemIndex(id)
	if !ok || index >= len(entries) ||
		entries[index].OperationID == "" || entries[index].DisabledReason != "" {
		return nil
	}
	m.MenuBar.selectItem(index)
	_, cmd := m.handleMenuBarKey(tea.KeyPressMsg{Code: tea.KeyEnter})
	return cmd
}

// relationPickerPointer is the list surfaces' shared answer inside the picker:
// a click seats the cursor, a double-click is Enter on the row, and the wheel
// steps the cursor. The picker has no Space, and while its search field is
// being edited Enter is the field's own, so a click there only seats. An event
// outside the popup does nothing, and so does a click on a popup fitting cut.
func (m *OutlinerModel) relationPickerPointer(event route.PointerEvent) tea.Cmd {
	state := &m.RelationPicker
	placement := m.modalFrame.picker
	if state.Surface == nil || !placement.Contains(event.X, event.Y) || (!placement.Whole && !event.IsWheel()) {
		return nil
	}
	key := state.Surface.AnswerPointer(
		event.Offset(placement.Col+chrome.OverlayBoxContentLeft, placement.Row+m.modalFrame.pickerListTop),
		false,
	)
	if state.Searching && key == "enter" {
		return nil
	}
	return route.PressKey(key)
}
