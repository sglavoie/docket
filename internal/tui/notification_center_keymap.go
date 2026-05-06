package tui

import (
	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/tui/keymap"
)

// buildKeyMap constructs the declarative KeyMap for NotificationCenterModel.
func (m *NotificationCenterModel) buildKeyMap() *keymap.KeyMap {
	var entries []keymap.Entry

	entries = append(entries,
		keymap.Bind("j", "Move cursor down", "Navigation", func() (tea.Model, tea.Cmd) {
			m.moveDown()
			return m, nil
		}, keymap.AsHidden()),
		keymap.Bind("down", "Move cursor down", "Navigation", func() (tea.Model, tea.Cmd) {
			m.moveDown()
			return m, nil
		}, keymap.AsHidden()),
		keymap.Bind("k", "Move cursor up", "Navigation", func() (tea.Model, tea.Cmd) {
			m.moveUp()
			return m, nil
		}, keymap.AsHidden()),
		keymap.Bind("up", "Move cursor up", "Navigation", func() (tea.Model, tea.Cmd) {
			m.moveUp()
			return m, nil
		}, keymap.AsHidden()),
		keymap.Bind("g", "Jump to first", "Navigation", func() (tea.Model, tea.Cmd) {
			for i, item := range m.items {
				if !item.isHeader {
					m.cursor = i
					break
				}
			}
			return m, nil
		}),
		keymap.Bind("G", "Jump to last", "Navigation", func() (tea.Model, tea.Cmd) {
			for i := len(m.items) - 1; i >= 0; i-- {
				if !m.items[i].isHeader {
					m.cursor = i
					break
				}
			}
			return m, nil
		}),
		keymap.Bind("n", "Next section", "Navigation", func() (tea.Model, tea.Cmd) {
			return m, m.SectionGoToNext(m.sectionStarts)
		}),
		keymap.Bind("N", "Previous section", "Navigation", func() (tea.Model, tea.Cmd) {
			return m, m.SectionGoToPrev(m.sectionStarts)
		}),
		keymap.Bind("tab", "Expand/collapse notification", "Actions", func() (tea.Model, tea.Cmd) {
			if m.cursor >= 0 && m.cursor < len(m.items) && !m.items[m.cursor].isHeader {
				if m.expandedItem == m.cursor {
					m.expandedItem = -1
				} else {
					m.expandedItem = m.cursor
				}
			}
			return m, nil
		}),
		keymap.Bind("d", "Clear/restore notification", "Actions", func() (tea.Model, tea.Cmd) {
			if a := m.selectedAlert(); a != nil {
				key := a.DismissalKey()
				cat := string(a.Category)
				if m.selectedItem().dismissed {
					return m, func() tea.Msg { return UndismissAlertMsg{Key: key} }
				}
				return m, func() tea.Msg { return DismissAlertMsg{Key: key, Category: cat} }
			}
			return m, nil
		}),
		keymap.Bind("D", "Toggle dismissed view", "Actions", func() (tea.Model, tea.Cmd) {
			m.showDismissed = !m.showDismissed
			m.buildItems()
			m.clampCursor()
			m.expandedItem = -1
			return m, nil
		}),
		keymap.Bind("o", "View task details", "Actions", func() (tea.Model, tea.Cmd) {
			if a := m.selectedAlert(); a != nil && a.TaskID != "" {
				taskID, projectID := a.TaskID, a.ProjectID
				return m, func() tea.Msg {
					return AlertCenterShowDetailMsg{TaskID: taskID, ProjectID: projectID}
				}
			}
			return m, nil
		}),
		keymap.Bind("enter", "Navigate to notification", "Actions", func() (tea.Model, tea.Cmd) {
			if a := m.selectedAlert(); a != nil {
				return m, m.navigateToAlert(*a)
			}
			return m, nil
		}),
	)

	return &keymap.KeyMap{
		ViewName: "Notification Center",
		Entries:  entries,
	}
}
