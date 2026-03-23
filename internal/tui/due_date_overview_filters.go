package tui

import (
	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
)

// refreshAndClamp reloads data and clamps cursor after filter changes.
func (m *DueDateOverviewModel) refreshAndClamp() { m.loadTasksWithDueDates(); m.clampCursor() }

func (m *DueDateOverviewModel) buildTagCmd() tea.Cmd {
	groups := [][]CrossProjectTask{m.overdue, m.today, m.thisWeek, m.later}
	var tasks []*models.Task
	for _, group := range groups {
		for i := range group {
			tasks = append(tasks, group[i].Task)
		}
	}
	return buildTagCmdFromTasks(tasks, m.filter.Tags, ViewDueDateOverview, "Filter by Tags")
}

// handleDueDateFilterPrefix handles the second key after pressing F (filter prefix) in due date overview.
func (m *DueDateOverviewModel) handleDueDateFilterPrefix(key string) (tea.Model, tea.Cmd) {
	if cmd, handled := handleFilterPrefixForView(m, key); handled {
		return m, cmd
	}
	return m, nil
}

// handleDueDateFilterSubPrefix handles all filter sub-prefix keys (Ft, Fs, Fa, Fas-Fap, FS).
func (m *DueDateOverviewModel) handleDueDateFilterSubPrefix(prefix, key string) (tea.Model, tea.Cmd) {
	handleFilterSubPrefixForView(m, prefix, key)
	return m, nil
}
