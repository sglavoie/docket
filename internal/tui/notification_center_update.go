package tui

import (
	tea "charm.land/bubbletea/v2"
)

// Update implements tea.Model.
func (m *NotificationCenterModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case CursorFlashClearMsg:
		m.cursorFlash = false
		return m, nil

	case tea.WindowSizeMsg:
		m.SetSize(msg.Width, msg.Height)

	case tea.KeyPressMsg:
		switch msg.String() {
		case "esc":
			return m, func() tea.Msg { return CloseNotificationCenterMsg{} }

		case "j", "down":
			m.moveDown()

		case "k", "up":
			m.moveUp()

		case "g":
			// Jump to first selectable
			for i, item := range m.items {
				if !item.isHeader {
					m.cursor = i
					break
				}
			}

		case "G":
			// Jump to last selectable
			for i := len(m.items) - 1; i >= 0; i-- {
				if !m.items[i].isHeader {
					m.cursor = i
					break
				}
			}

		case "n":
			return m, m.SectionGoToNext(m.sectionStarts)

		case "N":
			return m, m.SectionGoToPrev(m.sectionStarts)

		case "tab":
			if m.cursor >= 0 && m.cursor < len(m.items) && !m.items[m.cursor].isHeader {
				if m.expandedItem == m.cursor {
					m.expandedItem = -1
				} else {
					m.expandedItem = m.cursor
				}
			}

		case "d":
			if a := m.selectedAlert(); a != nil {
				key := a.DismissalKey()
				cat := string(a.Category)
				// If the selected alert is dismissed, un-dismiss it
				if m.selectedItem().dismissed {
					return m, func() tea.Msg {
						return UndismissAlertMsg{Key: key}
					}
				}
				return m, func() tea.Msg {
					return DismissAlertMsg{Key: key, Category: cat}
				}
			}

		case "D":
			m.showDismissed = !m.showDismissed
			m.buildItems()
			m.clampCursor()
			m.expandedItem = -1

		case "o":
			if a := m.selectedAlert(); a != nil && a.TaskID != "" {
				taskID, projectID := a.TaskID, a.ProjectID
				return m, func() tea.Msg {
					return AlertCenterShowDetailMsg{TaskID: taskID, ProjectID: projectID}
				}
			}

		case "enter":
			if a := m.selectedAlert(); a != nil {
				return m, m.navigateToAlert(*a)
			}
		}
	}

	return m, nil
}

// HandledKeys returns the list of keys this view handles.
func (m *NotificationCenterModel) HandledKeys() []string {
	return m.buildKeyMap().HandledKeys()
}

// ShouldCaptureEsc returns true so the view handles its own esc/q.
func (m *NotificationCenterModel) ShouldCaptureEsc() bool {
	return m.DefaultShouldCaptureEsc()
}
