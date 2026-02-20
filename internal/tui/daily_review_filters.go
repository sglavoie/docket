package tui

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/sglavoie/tt/internal/models"
)

// refreshAndClamp reloads data and clamps cursor after filter changes.
func (m *DailyReviewModel) refreshAndClamp() { m.loadTasks(); m.clampCursor() }

func (m *DailyReviewModel) buildTagCmd() tea.Cmd {
	sections := [][]DailyReviewTask{
		m.overdue, m.dueToday, m.dueSoon, m.dueLater,
		m.future, m.inProgress, m.blocked, m.inReview, m.old, m.completed,
	}
	var tasks []*models.Task
	for _, section := range sections {
		for i := range section {
			tasks = append(tasks, section[i].Task)
		}
	}
	tags := buildTagListFromTasks(tasks)
	return func() tea.Msg {
		return ShowTagFilterMsg{Tags: tags, Selected: m.filter.Tags, Source: ViewDailyReview, Title: "Filter by Tags"}
	}
}

// handleDailyReviewFilterPrefix handles the second key after pressing F (filter prefix) in daily review.
// Omits DueDate (sections group by due urgency) and Status (sections group by status).
func (m *DailyReviewModel) handleDailyReviewFilterPrefix(key string) (tea.Model, tea.Cmd) {
	if cmd, handled := handleFilterPrefixForView(m, key); handled {
		return m, cmd
	}
	return m, nil
}

// handleDailyReviewFilterSubPrefix handles all filter sub-prefix keys (Ft, Fa, Fas-Fap, FS).
func (m *DailyReviewModel) handleDailyReviewFilterSubPrefix(prefix, key string) (tea.Model, tea.Cmd) {
	handleFilterSubPrefixForView(m, prefix, key)
	return m, nil
}
