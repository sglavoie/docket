package tui

import (
	tea "charm.land/bubbletea/v2"
)

// handleKeyMsg handles keyboard input in normal mode.
func (m *FocusBoardModel) handleKeyMsg(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	if model, cmd, handled := m.Dispatcher.Dispatch(msg); handled {
		return model, cmd
	}
	return m, nil
}

// GetHelpItems returns help items for this view.
func (m *FocusBoardModel) GetHelpItems() []HelpItem {
	return mergeHelpItems(
		GetKeybindingRegistry().GetAppLevelForView(m.viewID()),
		helpItemsFromKeyMap(m.buildKeyMap()),
	)
}

// Multi-select helpers.

func (m *FocusBoardModel) toggleMultiSelectMode() {
	m.ToggleMode(&m.statusMessage, "Multi-select: space=toggle, a=all, A=deselect")
}

func (m *FocusBoardModel) toggleTaskSelection() {
	row := m.getRowAtCursor()
	if row == nil || !row.isTask() {
		return
	}
	m.ToggleAndUpdateStatus(row.Task.ID, &m.statusMessage, "Multi-select: space=toggle", "%d selected")
}

func (m *FocusBoardModel) selectAllVisible() {
	for _, row := range m.filtered {
		if row.isTask() {
			m.selectedTasks[row.Task.ID] = true
		}
	}
	m.UpdateStatus(&m.statusMessage, "Multi-select: space=toggle", "%d selected")
}

func (m *FocusBoardModel) deselectAll() {
	m.ClearSelection()
	m.UpdateStatus(&m.statusMessage, "Multi-select: space=toggle", "%d selected")
}

// getSelectedBulkItems returns selected tasks as bulkTaskItem slice.
func (m *FocusBoardModel) getSelectedBulkItems() []bulkTaskItem {
	var items []bulkTaskItem
	for _, row := range m.filtered {
		if row.isTask() && m.selectedTasks[row.Task.ID] {
			items = append(items, bulkTaskItem{Task: row.Task, Project: row.Project})
		}
	}
	return items
}

func (m *FocusBoardModel) HandledKeys() []string {
	return m.buildKeyMap().HandledKeys()
}
