package tui

import (
	tea "github.com/charmbracelet/bubbletea"
)

// handleSearchInput processes keyboard input while in search mode.
func (m *KanbanViewModel) handleSearchInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	handled, cmd, valueChanged := m.HandleSearchInput(msg)
	if handled || valueChanged {
		// Sync search query to filter and reload tasks
		m.filter.Search = m.SearchQuery()
		m.loadTasks()
	}
	return m, cmd
}

// handleDetailsPaneKey handles keys when the details pane has focus.
func (m *KanbanViewModel) handleDetailsPaneKey(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "esc":
		// Return focus to kanban pane
		m.detailsPaneFocused = false
		return m, nil

	case "q":
		// Close the kanban view entirely
		return m, func() tea.Msg {
			return CloseKanbanMsg{}
		}

	case "enter":
		// Open task detail view
		task := m.getSelectedTask()
		if task != nil {
			project := m.Project
			return m, func() tea.Msg {
				return ShowTaskDetailMsg{Task: task, Project: project}
			}
		}
		return m, nil

	case "j", "down":
		// Scroll panel content down
		m.detailsScroll++
		return m, nil

	case "k", "up":
		// Scroll panel content up
		if m.detailsScroll > 0 {
			m.detailsScroll--
		}
		return m, nil

	case "ctrl+d", "pgdown":
		// Scroll panel content down by half page
		halfPage := m.detailsHeight / 2
		if halfPage < 1 {
			halfPage = 1
		}
		m.detailsScroll += halfPage
		return m, nil

	case "ctrl+u", "pgup":
		// Scroll panel content up by half page
		halfPage := m.detailsHeight / 2
		if halfPage < 1 {
			halfPage = 1
		}
		m.detailsScroll -= halfPage
		if m.detailsScroll < 0 {
			m.detailsScroll = 0
		}
		return m, nil

	case "?":
		return m, helpCmd(ViewKanban, "Kanban", m.GetHelpItems())
	}

	// Other keys are no-op when details pane is focused
	return m, nil
}
