package tui

import (
	"fmt"
	"sort"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/icons"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/util"
)

// loadData aggregates projects and tasks into weekly review sections.
func (m *WeeklyReviewModel) loadData() { //nolint:gocyclo
	m.staleProjects = nil
	m.inboxBacklog = nil
	m.blockedWork = nil
	m.overcommitted = nil
	m.staleTasks = nil
	m.priorityMismatch = nil
	m.upcomingWeek = nil
	m.recurringOverview = nil
	m.priorityDriftWarning = ""
	m.priorityDriftPct = 0

	now := time.Now()
	today := util.StartOfDay(now)

	staleProjectDays := m.config.GetWeeklyStaleProjectDays()
	staleTaskDays := m.config.GetWeeklyStaleTaskDays()
	inboxThreshold := m.config.GetWeeklyInboxThreshold()
	blockedThreshold := m.config.GetWeeklyBlockedThreshold()
	inProgressThreshold := m.config.GetWeeklyInProgressThreshold()

	staleProjectThreshold := now.AddDate(0, 0, -staleProjectDays)
	staleTaskThreshold := now.AddDate(0, 0, -staleTaskDays)
	weekFromNow := today.AddDate(0, 0, 7) // Next 7 days (today..today+6), exclusive end

	var totalActive, highPriCount int

	projects := m.config.GetActiveProjects()
	for _, project := range projects {
		if err := storage.LoadProject(project); err != nil {
			continue
		}

		// Compute project metrics
		wp := WeeklyReviewProject{Project: project}
		var latestUpdate time.Time

		for _, task := range project.Tasks {
			if task.IsTerminalState() {
				continue
			}
			if task.IsSnoozed() {
				continue
			}

			totalActive++
			if task.Priority <= 2 {
				highPriCount++
			}

			// Track latest update (fall back to CreatedAt when UpdatedAt is zero)
			tu := task.UpdatedAt
			if tu.IsZero() {
				tu = task.CreatedAt
			}
			if tu.After(latestUpdate) {
				latestUpdate = tu
			}

			// Inbox count
			if task.IsInState(models.StateInbox) && task.IsRoot() {
				wp.InboxCount++
			}

			// Blocked count
			if task.IsInState(models.StateWaiting) || project.HasIncompleteDependencies(task) {
				wp.BlockedCount++
			}

			// In-progress count
			if task.IsInState(models.StateInProgress) {
				wp.InProgressCount++
			}

			// Overdue count
			if task.IsOverdue() {
				wp.OverdueCount++
			}

			// High priority todo
			if task.Priority <= 2 && taskInPlanningState(task) {
				wp.HighPriorityTodo++
			}

			if m.shouldSkipWeeklyReviewTask(task, project) {
				continue
			}

			// Task-level sections
			// Stale tasks (fall back to CreatedAt when UpdatedAt is zero)
			effectiveUpdate := task.UpdatedAt
			if effectiveUpdate.IsZero() {
				effectiveUpdate = task.CreatedAt
			}
			if !task.IsTerminalState() && effectiveUpdate.Before(staleTaskThreshold) {
				m.staleTasks = append(m.staleTasks, WeeklyReviewTask{Task: task, Project: project})
			}

			// Priority mismatch: high priority still in todo
			if task.Priority <= 2 && taskInPlanningState(task) {
				m.priorityMismatch = append(m.priorityMismatch, WeeklyReviewTask{Task: task, Project: project})
			}

			// Upcoming week
			if task.DueDate != nil && !task.IsTerminalState() {
				dueDay := util.StartOfDay(*task.DueDate)
				if !dueDay.Before(today) && dueDay.Before(weekFromNow) {
					m.upcomingWeek = append(m.upcomingWeek, WeeklyReviewTask{Task: task, Project: project})
				}
			}

			// Recurring overview
			if task.IsRecurring() {
				m.recurringOverview = append(m.recurringOverview, WeeklyReviewTask{Task: task, Project: project})
			}
		}

		// Calculate no-activity days
		if !latestUpdate.IsZero() {
			wp.NoActivityDays = util.DaysBetween(latestUpdate, now)
		} else {
			wp.NoActivityDays = util.DaysBetween(project.CreatedAt, now)
		}

		// Project-level sections
		if wp.NoActivityDays >= staleProjectDays && latestUpdate.Before(staleProjectThreshold) && len(project.Tasks) > 0 {
			m.staleProjects = append(m.staleProjects, wp)
		}
		if wp.InboxCount >= inboxThreshold {
			m.inboxBacklog = append(m.inboxBacklog, wp)
		}
		if wp.BlockedCount >= blockedThreshold {
			m.blockedWork = append(m.blockedWork, wp)
		}
		if wp.InProgressCount >= inProgressThreshold {
			m.overcommitted = append(m.overcommitted, wp)
		}
	}

	// Priority drift warning: flag when >40% of active tasks are P1/P2
	if totalActive > 0 {
		pct := highPriCount * 100 / totalActive
		if pct > 40 {
			m.priorityDriftPct = pct
			m.priorityDriftWarning = fmt.Sprintf(
				"%s  %d%% of active tasks are P1/P2 (%d of %d). Consider re-evaluating priorities.",
				icons.Get("warning"),
				pct, highPriCount, totalActive,
			)
		}
	}

	m.sortSections()
	m.calculateNavigation()

	// Auto-review empty sections
	allSections := []WeeklyReviewSection{
		WeeklySectionStaleProjects, WeeklySectionInboxBacklog,
		WeeklySectionBlockedWork, WeeklySectionOvercommitted,
		WeeklySectionStaleTasks, WeeklySectionPriorityMismatch,
		WeeklySectionUpcomingWeek, WeeklySectionRecurringOverview,
	}
	for _, s := range allSections {
		if m.getSectionItemCount(s) == 0 {
			m.reviewedSections[s] = true
		}
	}
}

func (m *WeeklyReviewModel) shouldSkipWeeklyReviewTask(task *models.Task, project *models.Project) bool {
	if task == nil {
		return true
	}
	if !hasActiveCommonFilters(&m.filter) {
		return false
	}
	return !passesCommonFilters(task, &m.filter, project, m.config)
}

// sortSections sorts each section by appropriate criteria.
func (m *WeeklyReviewModel) sortSections() {
	sortWeeklyProjectsByMetric(m.staleProjects, func(project WeeklyReviewProject) int {
		return project.NoActivityDays
	})
	sortWeeklyProjectsByMetric(m.inboxBacklog, func(project WeeklyReviewProject) int {
		return project.InboxCount
	})
	sortWeeklyProjectsByMetric(m.blockedWork, func(project WeeklyReviewProject) int {
		return project.BlockedCount
	})
	sortWeeklyProjectsByMetric(m.overcommitted, func(project WeeklyReviewProject) int {
		return project.InProgressCount
	})

	sortWeeklyTasks(m.staleTasks, staleWeeklyTaskLess)
	sortWeeklyTasks(m.priorityMismatch, priorityMismatchTaskLess)
	sortWeeklyTasks(m.upcomingWeek, dueDateThenPriorityTaskLess)
	sortWeeklyTasks(m.recurringOverview, dueDateThenPriorityTaskLess)
}

func sortWeeklyProjectsByMetric(projects []WeeklyReviewProject, metric func(WeeklyReviewProject) int) {
	sort.Slice(projects, func(i, j int) bool {
		left := metric(projects[i])
		right := metric(projects[j])
		if left != right {
			return left > right
		}
		return projects[i].Project.ID < projects[j].Project.ID
	})
}

func sortWeeklyTasks(tasks []WeeklyReviewTask, less func(left, right WeeklyReviewTask) bool) {
	sort.Slice(tasks, func(i, j int) bool {
		return less(tasks[i], tasks[j])
	})
}

func staleWeeklyTaskLess(left, right WeeklyReviewTask) bool {
	leftTime := weeklyTaskActivityTime(left)
	rightTime := weeklyTaskActivityTime(right)
	if !leftTime.Equal(rightTime) {
		return leftTime.Before(rightTime)
	}
	return left.Task.ID < right.Task.ID
}

func priorityMismatchTaskLess(left, right WeeklyReviewTask) bool {
	if left.Task.Priority != right.Task.Priority {
		return left.Task.Priority < right.Task.Priority
	}
	if cmp := compareWeeklyDueDates(left.Task.DueDate, right.Task.DueDate); cmp != 0 {
		return cmp < 0
	}
	return left.Task.ID < right.Task.ID
}

func dueDateThenPriorityTaskLess(left, right WeeklyReviewTask) bool {
	if cmp := compareWeeklyDueDates(left.Task.DueDate, right.Task.DueDate); cmp != 0 {
		return cmp < 0
	}
	if left.Task.Priority != right.Task.Priority {
		return left.Task.Priority < right.Task.Priority
	}
	return left.Task.ID < right.Task.ID
}

func weeklyTaskActivityTime(task WeeklyReviewTask) time.Time {
	if !task.Task.UpdatedAt.IsZero() {
		return task.Task.UpdatedAt
	}
	return task.Task.CreatedAt
}

func compareWeeklyDueDates(left, right *time.Time) int {
	switch {
	case left != nil && right == nil:
		return -1
	case left == nil && right != nil:
		return 1
	case left != nil && right != nil:
		switch {
		case left.Before(*right):
			return -1
		case right.Before(*left):
			return 1
		}
	}
	return 0
}

// toggleCurrentSection toggles collapse state for the section containing the cursor.
func (m *WeeklyReviewModel) toggleCurrentSection() (tea.Model, tea.Cmd) {
	if len(m.sectionOrder) == 0 {
		return m, nil
	}

	pos := 0
	for _, section := range m.sectionOrder {
		headerPos := pos
		pos++

		count := m.getSectionItemCount(section)
		var sectionEnd int
		if m.collapsedSections[int(section)] {
			sectionEnd = pos
		} else {
			sectionEnd = pos + count
		}

		if m.cursor >= headerPos && m.cursor < sectionEnd {
			ToggleSection(m.collapsedSections, int(section))
			m.calculateNavigation()
			m.clampCursor()
			return m, nil
		}

		pos = sectionEnd
	}

	return m, nil
}

// toggleSectionFocus toggles focus on a specific section.
func (m *WeeklyReviewModel) toggleSectionFocus(section WeeklyReviewSection) (tea.Model, tea.Cmd) {
	if m.focusedSection != nil && *m.focusedSection == section {
		m.focusedSection = nil
	} else {
		m.focusedSection = &section
	}
	m.calculateNavigation()
	m.clampCursor()
	return m, nil
}

// getTotalItemCount returns count across all sections.
func (m *WeeklyReviewModel) getTotalItemCount() int {
	return len(m.staleProjects) + len(m.inboxBacklog) + len(m.blockedWork) + len(m.overcommitted) +
		len(m.staleTasks) + len(m.priorityMismatch) + len(m.upcomingWeek) + len(m.recurringOverview)
}

// markCurrentSectionReviewed marks the section at the cursor as reviewed.
func (m *WeeklyReviewModel) markCurrentSectionReviewed() (tea.Model, tea.Cmd) {
	if len(m.sectionOrder) == 0 {
		return m, nil
	}
	idx := m.getCurrentSectionIndex()
	section := m.sectionOrder[idx]
	m.reviewedSections[section] = true
	m.setStatus("Marked \"" + getWeeklySectionName(section) + "\" as reviewed")
	if m.allSectionsReviewed() {
		m.reviewCompleted = true
		m.completedAt = time.Now()
		m.showingSummary = true
		return m, func() tea.Msg { return MarkWeeklyReviewDoneMsg{} }
	}
	return m, nil
}

// allSectionsReviewed returns true if all sections have been reviewed.
func (m *WeeklyReviewModel) allSectionsReviewed() bool {
	allSections := []WeeklyReviewSection{
		WeeklySectionStaleProjects, WeeklySectionInboxBacklog,
		WeeklySectionBlockedWork, WeeklySectionOvercommitted,
		WeeklySectionStaleTasks, WeeklySectionPriorityMismatch,
		WeeklySectionUpcomingWeek, WeeklySectionRecurringOverview,
	}
	for _, s := range allSections {
		if !m.reviewedSections[s] {
			return false
		}
	}
	return true
}

// reviewedCount returns the number of reviewed sections.
func (m *WeeklyReviewModel) reviewedCount() int {
	count := 0
	allSections := []WeeklyReviewSection{
		WeeklySectionStaleProjects, WeeklySectionInboxBacklog,
		WeeklySectionBlockedWork, WeeklySectionOvercommitted,
		WeeklySectionStaleTasks, WeeklySectionPriorityMismatch,
		WeeklySectionUpcomingWeek, WeeklySectionRecurringOverview,
	}
	for _, s := range allSections {
		if m.reviewedSections[s] {
			count++
		}
	}
	return count
}

// getWeeklySectionName returns a human-readable name for a weekly review section.
func getWeeklySectionName(section WeeklyReviewSection) string {
	switch section {
	case WeeklySectionStaleProjects:
		return "Stale Projects"
	case WeeklySectionInboxBacklog:
		return "Inbox Backlog"
	case WeeklySectionBlockedWork:
		return "Blocked Work"
	case WeeklySectionOvercommitted:
		return "Overcommitted"
	case WeeklySectionStaleTasks:
		return "Stale Tasks"
	case WeeklySectionPriorityMismatch:
		return "Priority Mismatch"
	case WeeklySectionUpcomingWeek:
		return "Upcoming Week"
	case WeeklySectionRecurringOverview:
		return "Recurring Overview"
	}
	return ""
}
