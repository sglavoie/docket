package tui

import tea "charm.land/bubbletea/v2"

// refreshAndClamp reloads data and clamps cursor after filter changes.
func (m *DailyReviewModel) refreshAndClamp() { m.loadTasks(); m.clampCursor() }

func (m *DailyReviewModel) buildTagCmd() tea.Cmd {
	sections := [][]CrossProjectTask{
		m.overdue, m.dueToday, m.dueSoon, m.dueLater,
		m.future, m.inProgress, m.blocked, m.inReview, m.inStandby, m.old, m.completed,
	}
	var items []CrossProjectTask
	for _, section := range sections {
		items = append(items, section...)
	}
	return buildTagCmdFromProjectTasks(
		projectsFromCrossProjectTasks(items),
		tasksFromCrossProjectTasks(items),
		m.filter.Tags,
		ViewDailyReview,
		"Filter by Tags",
		false,
	)
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
