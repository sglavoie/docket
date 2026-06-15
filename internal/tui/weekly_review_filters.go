package tui

import tea "charm.land/bubbletea/v2"

// refreshAndClamp reloads data and clamps cursor after filter changes.
func (m *WeeklyReviewModel) refreshAndClamp() { m.loadData(); m.clampCursor() }

func (m *WeeklyReviewModel) buildTagCmd() tea.Cmd {
	taskSections := [][]WeeklyReviewTask{
		m.staleTasks, m.priorityMismatch, m.upcomingWeek, m.recurringOverview,
	}
	var items []WeeklyReviewTask
	for _, section := range taskSections {
		items = append(items, section...)
	}
	return buildTagCmdFromProjectTasks(
		projectsFromWeeklyReviewTasks(items),
		tasksFromWeeklyReviewTasks(items),
		m.filter.Tags,
		ViewWeeklyReview,
		"Filter by Tags",
		false,
	)
}

// handleWeeklyReviewFilterPrefix handles the second key after pressing F (filter prefix) in weekly review.
func (m *WeeklyReviewModel) handleWeeklyReviewFilterPrefix(key string) (tea.Model, tea.Cmd) {
	if cmd, handled := handleFilterPrefixForView(m, key); handled {
		return m, cmd
	}
	return m, nil
}

// handleWeeklyReviewFilterSubPrefix handles all filter sub-prefix keys (Ft, Fs, Fd, Fa, FS).
func (m *WeeklyReviewModel) handleWeeklyReviewFilterSubPrefix(prefix, key string) (tea.Model, tea.Cmd) {
	handleFilterSubPrefixForView(m, prefix, key)
	return m, nil
}
