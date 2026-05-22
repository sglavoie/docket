package tui

import (
	tea "charm.land/bubbletea/v2"
)

func (m *PriorityViewModel) handleKeypress(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	// Handle search input mode before dispatcher
	if m.IsSearchMode() {
		handled, cmd, valueChanged := m.HandleSearchInput(msg)
		if valueChanged || handled {
			m.recalculateNavigation()
			m.clampCursor()
		}
		return m, cmd
	}
	if model, cmd, handled := m.Dispatcher.Dispatch(msg); handled {
		return model, cmd
	}
	return m, nil
}

// getSelectedBulkItems returns selected tasks as bulkTaskItem slice.
func (m *PriorityViewModel) getSelectedBulkItems() []bulkTaskItem {
	var items []bulkTaskItem
	for _, group := range [][]*PriorityTask{m.overdue, m.today, m.thisWeek, m.later, m.backlog, m.waitingOn} {
		for _, pt := range group {
			if m.selectedTasks[pt.Task.ID] {
				items = append(items, bulkTaskItem{Task: pt.Task, Project: m.project})
			}
		}
	}
	return items
}
