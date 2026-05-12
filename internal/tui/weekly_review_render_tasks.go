package tui

import "strings"

// renderTaskLine renders a single task line (same style as daily review).
func (m *WeeklyReviewModel) renderTaskLine(dt WeeklyReviewTask, section WeeklyReviewSection, isSelected bool, maxTitleLen int, layout *ColumnLayout, rowIndex int) string {
	task := dt.Task

	// Context string based on section
	contextStr := ""
	switch section {
	case WeeklySectionStaleTasks:
		t := task.UpdatedAt
		if t.IsZero() {
			t = task.CreatedAt
		}
		if t.IsZero() {
			contextStr = MutedStyle.Render("no date")
		} else {
			contextStr = MutedStyle.Render(formatRelativeTime(t))
		}
	case WeeklySectionPriorityMismatch:
		if task.DueDate != nil {
			contextStr = DueDateStyle.Render(task.DueDate.Format("Jan 2"))
		} else {
			contextStr = MutedStyle.Render("no due date")
		}
	case WeeklySectionUpcomingWeek:
		if task.DueDate != nil {
			contextStr = DueDateStyle.Render(task.DueDate.Format("Mon Jan 2"))
		}
	case WeeklySectionRecurringOverview:
		if task.DueDate != nil {
			contextStr = DueDateStyle.Render(task.DueDate.Format("Jan 2"))
		}
		if dates := task.NextNDueDates(2); len(dates) > 0 {
			var upcoming []string
			for _, d := range dates {
				upcoming = append(upcoming, d.Format("Jan 2"))
			}
			nextStr := MutedStyle.Render("→ " + strings.Join(upcoming, ", "))
			if contextStr != "" {
				contextStr += " " + nextStr
			} else {
				contextStr = nextStr
			}
		}
	default:
	}

	return RenderCrossProjectTaskLine(task, dt.Project, m.config, isSelected, m.width, maxTitleLen, CrossProjectTaskLineOpts{
		Queued:          m.queueIDs[task.ID],
		DailyPlanned:    m.dailyPlanIDs[task.ID],
		HideLowPriority: true,
		DueStr:          contextStr,
		ShowRecurrence:  section == WeeklySectionRecurringOverview,
		Prefix:          m.Checkbox(task.ID),
		Flash:           m.cursorFlash,
		Layout:          layout,
		RowIndex:        rowIndex,
	})
}

// getWeeklySectionHint returns a short action hint for a section.
func getWeeklySectionHint(section WeeklyReviewSection) string {
	switch section {
	case WeeklySectionStaleProjects:
		return "Archive or review tasks in these projects"
	case WeeklySectionInboxBacklog:
		return "Process inbox items to keep backlogs manageable"
	case WeeklySectionBlockedWork:
		return "Unblock tasks or reassess blocked work"
	case WeeklySectionOvercommitted:
		return "Reduce WIP — complete or defer in-progress tasks"
	case WeeklySectionStaleTasks:
		return "Update, snooze, or close these stale tasks"
	case WeeklySectionPriorityMismatch:
		return "Start high-priority tasks or lower their priority"
	case WeeklySectionUpcomingWeek:
		return "Confirm these tasks are ready for the coming week"
	case WeeklySectionRecurringOverview:
		return "Review recurring tasks — skip, defer, or adjust patterns"
	}
	return ""
}
