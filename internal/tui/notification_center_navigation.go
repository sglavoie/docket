package tui

import (
	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/alerts"
)

// moveDown moves cursor to the next selectable item.
func (m *NotificationCenterModel) moveDown() {
	for i := m.cursor + 1; i < len(m.items); i++ {
		if !m.items[i].isHeader {
			m.cursor = i
			m.expandedItem = -1
			return
		}
	}
}

// moveUp moves cursor to the previous selectable item.
func (m *NotificationCenterModel) moveUp() {
	for i := m.cursor - 1; i >= 0; i-- {
		if !m.items[i].isHeader {
			m.cursor = i
			m.expandedItem = -1
			return
		}
	}
}

// selectedItem returns the currently selected item, or nil if out of bounds.
func (m *NotificationCenterModel) selectedItem() *alertCenterItem {
	if m.cursor < 0 || m.cursor >= len(m.items) {
		return nil
	}
	return &m.items[m.cursor]
}

// clampCursor ensures the cursor is on a valid selectable item.
func (m *NotificationCenterModel) clampCursor() {
	if len(m.items) == 0 {
		m.cursor = 0
		return
	}
	// If current position is valid and selectable, keep it
	if m.cursor >= 0 && m.cursor < len(m.items) && !m.items[m.cursor].isHeader {
		return
	}
	// Find nearest selectable item (prefer moving up, then down)
	start := m.cursor - 1
	if start >= len(m.items) {
		start = len(m.items) - 1
	}
	for i := start; i >= 0; i-- {
		if !m.items[i].isHeader {
			m.cursor = i
			return
		}
	}
	for i := 0; i < len(m.items); i++ {
		if !m.items[i].isHeader {
			m.cursor = i
			return
		}
	}
	m.cursor = 0
}

// selectedAlert returns the currently selected alert, or nil if on a header.
func (m *NotificationCenterModel) selectedAlert() *alerts.Alert {
	item := m.selectedItem()
	if item == nil || item.isHeader {
		return nil
	}
	return &item.alert
}

// navigateToAlert returns a command that dispatches navigation to the alert's task/project.
func (m *NotificationCenterModel) navigateToAlert(a alerts.Alert) tea.Cmd {
	return func() tea.Msg {
		return AlertCenterNavigateMsg{
			TaskID:    a.TaskID,
			ProjectID: a.ProjectID,
			Category:  a.Category,
		}
	}
}
