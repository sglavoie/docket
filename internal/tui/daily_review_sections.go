package tui

import (
	"sort"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/util"
)

// loadTasks aggregates tasks from all active projects into sections.
func (m *DailyReviewModel) loadTasks() {
	m.overdue = nil
	m.dueToday = nil
	m.dueSoon = nil
	m.dueLater = nil
	m.future = nil
	m.inProgress = nil
	m.blocked = nil
	m.inReview = nil
	m.inStandby = nil
	m.old = nil
	m.completed = nil

	now := time.Now()
	today := util.StartOfDay(now)

	// Get configurable thresholds
	dueSoonDays := m.config.GetReviewDueSoonDays()
	futureDays := m.config.GetReviewFutureDays()
	oldDays := m.config.GetReviewOldDays()
	completedHours := m.config.GetReviewCompletedHours()

	// Calculate threshold dates
	// Due Soon: tomorrow through dueSoonDays (e.g., days 1-7)
	dueSoonEnd := today.AddDate(0, 0, dueSoonDays+1)
	// Future: tasks due in >= futureDays (e.g., 300+ days)
	futureStart := today.AddDate(0, 0, futureDays)
	// Due Later: between Due Soon and Future (e.g., days 8-299)
	oldThreshold := now.AddDate(0, 0, -oldDays)
	completedThreshold := now.Add(-time.Duration(completedHours) * time.Hour)

	projects := m.config.GetActiveProjects()
	for _, project := range projects {
		if err := storage.LoadProject(project); err != nil {
			continue
		}

		for _, task := range project.Tasks {
			if task == nil {
				continue
			}
			// Apply common filters (no DueDate or Status — sections group by these)
			if hasActiveCommonFilters(&m.filter) && !passesCommonFilters(task, &m.filter, project.ID, m.config) {
				continue
			}

			dt := CrossProjectTask{Task: task, Project: project}

			// 1. Completed: CompletedAt within completedHours
			if task.Status.IsComplete() && task.CompletedAt != nil {
				if completedHours > 0 && task.CompletedAt.After(completedThreshold) {
					m.completed = append(m.completed, dt)
				}
				continue
			}

			// 2. Skip other completed tasks
			if task.Status.IsComplete() {
				continue
			}

			// Skip snoozed tasks
			if task.IsSnoozed() {
				continue
			}

			// 3. On Standby: tasks in the Standby section or with standby status
			if task.Section == models.SectionStandby || task.Status == models.StatusStandby {
				m.inStandby = append(m.inStandby, dt)
				continue
			}

			// 4. In Review: tasks in the Review section or with review status
			if task.Section == models.SectionReview || task.Status == models.StatusReview {
				m.inReview = append(m.inReview, dt)
				continue
			}

			// 4. Overdue: DueDate < today
			if task.DueDate != nil {
				dueDay := util.StartOfDay(*task.DueDate)

				if dueDay.Before(today) {
					m.overdue = append(m.overdue, dt)
					continue
				}

				// 4. Today: DueDate == today
				if dueDay.Equal(today) {
					m.dueToday = append(m.dueToday, dt)
					continue
				}

				// 5. Due Soon: DueDate between tomorrow and dueSoonDays (e.g., 1-7 days)
				if dueSoonDays > 0 && dueDay.Before(dueSoonEnd) {
					m.dueSoon = append(m.dueSoon, dt)
					continue
				}

				// 6. Future: DueDate >= futureDays (e.g., 300+ days)
				if futureDays > 0 && !dueDay.Before(futureStart) {
					m.future = append(m.future, dt)
					continue
				}

				// 7. Due Later: DueDate between dueSoonDays+1 and futureDays-1 (e.g., 8-299 days)
				// This catches everything between Due Soon and Future
				m.dueLater = append(m.dueLater, dt)
				continue
			}

			// 8. Blocked: status == blocked or has incomplete deps
			if task.Status == models.StatusBlocked || project.HasIncompleteDependencies(task) {
				m.blocked = append(m.blocked, dt)
				continue
			}

			// 9. In Progress: status == in_progress
			if task.Status == models.StatusInProgress {
				m.inProgress = append(m.inProgress, dt)
				continue
			}

			// 10. Old: UpdatedAt older than oldDays (fall back to CreatedAt when UpdatedAt is zero)
			effectiveUpdate := task.UpdatedAt
			if effectiveUpdate.IsZero() {
				effectiveUpdate = task.CreatedAt
			}
			if oldDays > 0 && effectiveUpdate.Before(oldThreshold) {
				m.old = append(m.old, dt)
			}
		}
	}

	// Sort sections
	m.sortSections()
	m.calculateNavigation()
}

// crossTaskLess comparators for recurring sort patterns on CrossProjectTask slices.
func byDueDateThenPriority(a, b CrossProjectTask) bool {
	if !a.Task.DueDate.Equal(*b.Task.DueDate) {
		return a.Task.DueDate.Before(*b.Task.DueDate)
	}
	return a.Task.Priority < b.Task.Priority
}

func byPriorityThenID(a, b CrossProjectTask) bool {
	if a.Task.Priority != b.Task.Priority {
		return a.Task.Priority < b.Task.Priority
	}
	return a.Task.ID < b.Task.ID
}

func byPriorityThenUpdatedAt(a, b CrossProjectTask) bool {
	if a.Task.Priority != b.Task.Priority {
		return a.Task.Priority < b.Task.Priority
	}
	return a.Task.UpdatedAt.After(b.Task.UpdatedAt)
}

// sortSections sorts each section by appropriate criteria.
func (m *DailyReviewModel) sortSections() {
	// Overdue: oldest first (most urgent)
	sort.Slice(m.overdue, func(i, j int) bool {
		return m.overdue[i].Task.DueDate.Before(*m.overdue[j].Task.DueDate)
	})

	sort.Slice(m.dueToday, func(i, j int) bool { return byPriorityThenID(m.dueToday[i], m.dueToday[j]) })
	sort.Slice(m.dueSoon, func(i, j int) bool { return byDueDateThenPriority(m.dueSoon[i], m.dueSoon[j]) })
	sort.Slice(m.dueLater, func(i, j int) bool { return byDueDateThenPriority(m.dueLater[i], m.dueLater[j]) })
	sort.Slice(m.future, func(i, j int) bool { return byDueDateThenPriority(m.future[i], m.future[j]) })
	sort.Slice(m.inProgress, func(i, j int) bool { return byPriorityThenUpdatedAt(m.inProgress[i], m.inProgress[j]) })
	sort.Slice(m.blocked, func(i, j int) bool { return byPriorityThenID(m.blocked[i], m.blocked[j]) })
	sort.Slice(m.inReview, func(i, j int) bool { return byPriorityThenUpdatedAt(m.inReview[i], m.inReview[j]) })
	sort.Slice(m.inStandby, func(i, j int) bool { return byPriorityThenUpdatedAt(m.inStandby[i], m.inStandby[j]) })

	// Old: by UpdatedAt (oldest first), falling back to CreatedAt when zero
	sort.Slice(m.old, func(i, j int) bool {
		ti := m.old[i].Task.UpdatedAt
		if ti.IsZero() {
			ti = m.old[i].Task.CreatedAt
		}
		tj := m.old[j].Task.UpdatedAt
		if tj.IsZero() {
			tj = m.old[j].Task.CreatedAt
		}
		return ti.Before(tj)
	})

	// Completed: most recent first
	sort.Slice(m.completed, func(i, j int) bool {
		if m.completed[i].Task.CompletedAt == nil {
			return false
		}
		if m.completed[j].Task.CompletedAt == nil {
			return true
		}
		return m.completed[i].Task.CompletedAt.After(*m.completed[j].Task.CompletedAt)
	})
}

// toggleCurrentSection toggles collapse state for the section containing the cursor.
func (m *DailyReviewModel) toggleCurrentSection() (tea.Model, tea.Cmd) {
	if len(m.sectionOrder) == 0 {
		return m, nil
	}

	// Find which section contains the cursor (header or task within it)
	pos := 0
	for _, section := range m.sectionOrder {
		headerPos := pos
		pos++ // Move past header

		tasks := m.getSectionTasks(section)
		var sectionEnd int
		if m.collapsedSections[int(section)] {
			sectionEnd = pos // Collapsed: only header
		} else {
			sectionEnd = pos + len(tasks)
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
// If already focused on that section, shows all sections.
// Otherwise, focuses only on that section.
func (m *DailyReviewModel) toggleSectionFocus(section DailyReviewSection) (tea.Model, tea.Cmd) {
	if m.focusedSection != nil && *m.focusedSection == section {
		// Already focused on this section, show all
		m.focusedSection = nil
	} else {
		// Focus on this section
		m.focusedSection = &section
	}
	m.calculateNavigation()
	m.clampCursor()
	return m, nil
}

// getTotalTaskCount returns count across all sections (ignoring collapse state).
func (m *DailyReviewModel) getTotalTaskCount() int {
	return len(m.overdue) + len(m.dueToday) + len(m.dueSoon) + len(m.dueLater) +
		len(m.future) + len(m.inProgress) + len(m.blocked) + len(m.inReview) + len(m.inStandby) + len(m.old) + len(m.completed)
}

// getSectionName returns a human-readable name for a section.
func getSectionName(section DailyReviewSection) string {
	switch section {
	case SectionOverdue:
		return "Overdue"
	case SectionDueToday:
		return "Today"
	case SectionDueSoon:
		return "Due Soon"
	case SectionDueLater:
		return "Due Later"
	case SectionFuture:
		return "Future"
	case SectionInProgress:
		return "In Progress"
	case SectionBlocked:
		return "Blocked"
	case SectionInReview:
		return "In Review"
	case SectionOnStandby:
		return "On Standby"
	case SectionOld:
		return "Old"
	case SectionCompleted:
		return "Completed"
	}
	return ""
}
