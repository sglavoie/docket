package tui

import (
	"strings"

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
		// Open task detail view via glow
		task := m.getSelectedTask()
		if task != nil {
			return m, func() tea.Msg {
				return GlowViewMsg{Task: task}
			}
		}
		return m, nil

	case "j", "down":
		// Scroll description down
		task := m.getSelectedTask()
		if task != nil && task.Description != "" {
			descLines := strings.Split(task.Description, "\n")
			maxScroll := len(descLines) - 3 // Keep at least 3 lines visible
			if maxScroll < 0 {
				maxScroll = 0
			}
			if m.detailsScroll < maxScroll {
				m.detailsScroll++
			}
		}
		return m, nil

	case "k", "up":
		// Scroll description up
		if m.detailsScroll > 0 {
			m.detailsScroll--
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
