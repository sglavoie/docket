package tui

import (
	tea "github.com/charmbracelet/bubbletea"
)

// handleSearchInput processes keyboard input while in search mode.
func (m *KanbanViewModel) handleSearchInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	model, cmd, handled := m.HandleSearchMode(msg, m, func() {
		m.filter.Search = m.SearchQuery()
		m.loadTasks()
	})
	if handled {
		return model, cmd
	}
	return m, cmd
}

// handleDetailsPaneKey handles keys when the details pane has focus.
func (m *KanbanViewModel) handleDetailsPaneKey(key string) (tea.Model, tea.Cmd) {
	// Scroll keys handled by shared state
	if m.detailsPane.HandleScrollKey(key) {
		return m, nil
	}

	switch key {
	case "esc":
		m.detailsPane.Focused = false
		return m, nil

	case "q":
		return m, func() tea.Msg {
			return CloseKanbanMsg{}
		}

	case "enter":
		task := m.getSelectedTask()
		if task != nil {
			project := m.Project
			return m, func() tea.Msg {
				return ShowTaskDetailMsg{Task: task, Project: project}
			}
		}
		return m, nil

	case "?":
		return m, helpCmd(ViewKanban, "Kanban", m.GetHelpItems())
	}

	return m, nil
}
