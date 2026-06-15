package tui

import (
	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
)

// refreshAndClamp reloads data and clamps cursor after filter changes.
func (m *RecentActivityModel) refreshAndClamp() { m.applyFilter(); m.clampCursor() }

func (m *RecentActivityModel) buildTagCmd() tea.Cmd {
	if m.projectScope != nil {
		return buildTagCmdFromProjectTasks(
			[]*models.Project{m.projectScope},
			tasksFromCrossProjectTasks(m.tasks),
			m.filter.Tags,
			ViewRecentActivity,
			"Filter by Tags",
			false,
		)
	}
	return buildTagCmdFromProjectTasks(
		projectsFromCrossProjectTasks(m.tasks),
		tasksFromCrossProjectTasks(m.tasks),
		m.filter.Tags,
		ViewRecentActivity,
		"Filter by Tags",
		false,
	)
}

// handleRecentActivityFilterPrefix handles the second key after pressing F (filter prefix) in recent activity.
// NOTE: With Dispatcher wiring, this is only called from the keymap actions (not directly from prefix dispatch).
func (m *RecentActivityModel) handleRecentActivityFilterPrefix(key string) (tea.Model, tea.Cmd) {
	if cmd, handled := handleFilterPrefixForView(m, key); handled {
		return m, cmd
	}
	return m, nil
}

// handleRecentActivityFilterSubPrefix handles all filter sub-prefix keys (Ft, Fs, Fd, Fa, Fas-Fap, FS).
func (m *RecentActivityModel) handleRecentActivityFilterSubPrefix(prefix, key string) (tea.Model, tea.Cmd) {
	handleFilterSubPrefixForView(m, prefix, key)
	return m, nil
}
