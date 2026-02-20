package tui

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/sglavoie/tt/internal/models"
)

// refreshAndClamp reloads data and clamps cursor after filter changes.
func (m *DueDateOverviewModel) refreshAndClamp() { m.loadTasksWithDueDates(); m.clampCursor() }

func (m *DueDateOverviewModel) buildTagCmd() tea.Cmd {
	groups := [][]DueDateTask{m.overdue, m.today, m.thisWeek, m.later}
	var tasks []*models.Task
	for _, group := range groups {
		for i := range group {
			tasks = append(tasks, group[i].Task)
		}
	}
	tags := buildTagListFromTasks(tasks)
	return func() tea.Msg {
		return ShowTagFilterMsg{Tags: tags, Selected: m.filter.Tags, Source: ViewDueDateOverview, Title: "Filter by Tags"}
	}
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
