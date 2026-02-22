package tui

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/sglavoie/tt/internal/models"
)

// refreshAndClamp reloads data and clamps cursor after filter changes.
func (m *RecentActivityModel) refreshAndClamp() { m.applyFilter(); m.clampCursor() }

func (m *RecentActivityModel) buildTagCmd() tea.Cmd {
	tasks := make([]*models.Task, len(m.tasks))
	for i := range m.tasks {
		tasks[i] = m.tasks[i].Task
	}
	return buildTagCmdFromTasks(tasks, m.filter.Tags, ViewRecentActivity, "Filter by Tags")
}

// handleRecentActivityFilterPrefix handles the second key after pressing F (filter prefix) in recent activity.
func (m *RecentActivityModel) handleRecentActivityFilterPrefix(key string) (tea.Model, tea.Cmd) {
	// View-specific: "d" opens the due-date sub-prefix
	if key == "d" {
		m.pendingPrefix = "Fd"
		return m, nil
	}

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
