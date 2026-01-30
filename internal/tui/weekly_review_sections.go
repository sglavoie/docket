package tui

import (
	"sort"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

// loadData aggregates projects and tasks into weekly review sections.
func (m *WeeklyReviewModel) loadData() {
	m.staleProjects = nil
	m.inboxBacklog = nil
	m.blockedWork = nil
	m.overcommitted = nil
	m.staleTasks = nil
	m.priorityMismatch = nil
	m.upcomingWeek = nil

	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	staleProjectDays := m.config.GetWeeklyStaleProjectDays()
	staleTaskDays := m.config.GetWeeklyStaleTaskDays()
	inboxThreshold := m.config.GetWeeklyInboxThreshold()
	blockedThreshold := m.config.GetWeeklyBlockedThreshold()
	inProgressThreshold := m.config.GetWeeklyInProgressThreshold()

	staleProjectThreshold := now.AddDate(0, 0, -staleProjectDays)
	staleTaskThreshold := now.AddDate(0, 0, -staleTaskDays)
	weekFromNow := today.AddDate(0, 0, 8) // Next 7 days (exclusive end)

	projects := m.config.GetActiveProjects()
	for _, project := range projects {
		if err := storage.LoadProject(project); err != nil {
			continue
		}

		// Compute project metrics
		wp := WeeklyReviewProject{Project: project}
		var latestUpdate time.Time

		for _, task := range project.Tasks {
			if task.Status.IsComplete() {
				continue
			}
			if task.IsSnoozed() {
				continue
			}

			// Track latest update
			if task.UpdatedAt.After(latestUpdate) {
				latestUpdate = task.UpdatedAt
			}

			// Inbox count
			if task.Section == models.SectionInbox {
				wp.InboxCount++
			}

			// Blocked count
			if task.Status == models.StatusBlocked || project.HasIncompleteDependencies(task) {
				wp.BlockedCount++
			}

			// In-progress count
			if task.Status == models.StatusInProgress {
				wp.InProgressCount++
			}

			// Overdue count
			if task.IsOverdue() {
				wp.OverdueCount++
			}

			// High priority todo
			if task.Priority <= 2 && task.Status == models.StatusTodo {
				wp.HighPriorityTodo++
			}

			// Task-level sections
			// Stale tasks
			if !task.Status.IsComplete() && task.UpdatedAt.Before(staleTaskThreshold) {
				m.staleTasks = append(m.staleTasks, WeeklyReviewTask{Task: task, Project: project})
			}

			// Priority mismatch: high priority still in todo
			if task.Priority <= 2 && task.Status == models.StatusTodo {
				m.priorityMismatch = append(m.priorityMismatch, WeeklyReviewTask{Task: task, Project: project})
			}

			// Upcoming week
			if task.DueDate != nil && !task.Status.IsComplete() {
				dueDay := time.Date(task.DueDate.Year(), task.DueDate.Month(), task.DueDate.Day(), 0, 0, 0, 0, now.Location())
				if !dueDay.Before(today) && dueDay.Before(weekFromNow) {
					m.upcomingWeek = append(m.upcomingWeek, WeeklyReviewTask{Task: task, Project: project})
				}
			}
		}

		// Calculate no-activity days
		if !latestUpdate.IsZero() {
			wp.NoActivityDays = int(now.Sub(latestUpdate).Hours() / 24)
		} else {
			wp.NoActivityDays = int(now.Sub(project.CreatedAt).Hours() / 24)
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

	m.sortSections()
	m.calculateNavigation()
}

// sortSections sorts each section by appropriate criteria.
func (m *WeeklyReviewModel) sortSections() {
	// Stale projects: oldest first
	sort.Slice(m.staleProjects, func(i, j int) bool {
		return m.staleProjects[i].NoActivityDays > m.staleProjects[j].NoActivityDays
	})

	// Inbox backlog: by count desc
	sort.Slice(m.inboxBacklog, func(i, j int) bool {
		return m.inboxBacklog[i].InboxCount > m.inboxBacklog[j].InboxCount
	})

	// Blocked work: by count desc
	sort.Slice(m.blockedWork, func(i, j int) bool {
		return m.blockedWork[i].BlockedCount > m.blockedWork[j].BlockedCount
	})

	// Overcommitted: by count desc
	sort.Slice(m.overcommitted, func(i, j int) bool {
		return m.overcommitted[i].InProgressCount > m.overcommitted[j].InProgressCount
	})

	// Stale tasks: oldest first
	sort.Slice(m.staleTasks, func(i, j int) bool {
		return m.staleTasks[i].Task.UpdatedAt.Before(m.staleTasks[j].Task.UpdatedAt)
	})

	// Priority mismatch: by priority then due date
	sort.Slice(m.priorityMismatch, func(i, j int) bool {
		if m.priorityMismatch[i].Task.Priority != m.priorityMismatch[j].Task.Priority {
			return m.priorityMismatch[i].Task.Priority < m.priorityMismatch[j].Task.Priority
		}
		// Tasks with due dates come first
		if m.priorityMismatch[i].Task.DueDate != nil && m.priorityMismatch[j].Task.DueDate == nil {
			return true
		}
		if m.priorityMismatch[i].Task.DueDate == nil && m.priorityMismatch[j].Task.DueDate != nil {
			return false
		}
		if m.priorityMismatch[i].Task.DueDate != nil && m.priorityMismatch[j].Task.DueDate != nil {
			return m.priorityMismatch[i].Task.DueDate.Before(*m.priorityMismatch[j].Task.DueDate)
		}
		return false
	})

	// Upcoming week: by due date then priority
	sort.Slice(m.upcomingWeek, func(i, j int) bool {
		if m.upcomingWeek[i].Task.DueDate != nil && m.upcomingWeek[j].Task.DueDate != nil {
			if !m.upcomingWeek[i].Task.DueDate.Equal(*m.upcomingWeek[j].Task.DueDate) {
				return m.upcomingWeek[i].Task.DueDate.Before(*m.upcomingWeek[j].Task.DueDate)
			}
		}
		return m.upcomingWeek[i].Task.Priority < m.upcomingWeek[j].Task.Priority
	})
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
			m.collapsedSections[int(section)] = !m.collapsedSections[int(section)]
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
		len(m.staleTasks) + len(m.priorityMismatch) + len(m.upcomingWeek)
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
	}
	return ""
}
