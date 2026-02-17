package tui

import (
	tea "github.com/charmbracelet/bubbletea"
)

// handleSearchInput processes keyboard input while in search mode.
func (m *KanbanViewModel) handleSearchInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		// Cancel search
		m.searchMode = false
		m.searchInput.Blur()
		m.searchInput.SetValue("")
		return m, nil

	case "enter":
		// Confirm search
		m.searchMode = false
		m.searchInput.Blur()
		m.filter.Search = m.searchInput.Value()
		m.loadTasks()
		return m, nil

	default:
		// Update search input
		var cmd tea.Cmd
		m.searchInput, cmd = m.searchInput.Update(msg)
		// Live filtering as user types
		m.filter.Search = m.searchInput.Value()
		m.loadTasks()
		return m, cmd
	}
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
		return m, func() tea.Msg {
			return ShowHelpMsg{
				SourceView: ViewKanban,
				ViewName:   "Kanban",
				Items:      m.GetHelpItems(),
			}
		}
	}

	// Other keys are no-op when details pane is focused
	return m, nil
}
