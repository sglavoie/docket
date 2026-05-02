package tui

import (
	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
)

// handleNavigationPrefix handles the g prefix key for navigation operations.
func (m *OutlinerModel) handleNavigationPrefix(key string, cursorTask *models.Task) (tea.Model, tea.Cmd) {
	switch key {
	case "g":
		old := m.cursor
		m.cursor = 0
		return m, flashIfJumped(old, m.cursor, &m.cursorFlash)
	case "h":
		if m.project == nil {
			return m, nil
		}
		if cursorTask != nil {
			task := cursorTask
			project := m.project
			return m, func() tea.Msg {
				return OpenTaskActivityMsg{Task: task, Project: project}
			}
		}
		return m, nil
	case "X":
		// Deselect all visible in multi-select mode
		if m.multiSelectMode {
			m.deselectAllVisible()
		}
		return m, nil
	case "d":
		// Cycle due date filter
		m.cycleDueDateFilter()
		m.rebuildRows()
		return m, nil
	case "z":
		// Jump directly to Snoozed perspective (P0)
		m.activePerspective = PerspectiveSnoozed
		m.rebuildRows()
		return m, nil
	case "l":
		if m.project == nil {
			return m, nil
		}
		// Go to project recent activity
		project := m.project
		return m, func() tea.Msg {
			return ShowProjectRecentActivityMsg{Project: project}
		}
	case "o":
		// Drill down to the single-project outliner for the cursor task's project
		if m.project == nil {
			return m.crossHandleGoDrillDown()
		}
		return m, nil
	case "k":
		// Open kanban view for this project (or cross-project kanban)
		if m.project == nil {
			return m, func() tea.Msg { return ShowCrossProjectKanbanMsg{} }
		}
		project := m.project
		return m, func() tea.Msg { return ShowKanbanMsg{Project: project} }
	}

	return m, nil
}
