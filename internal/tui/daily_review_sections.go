package tui

import (
	"sort"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/util"
)

// loadTasks aggregates tasks from all active projects into sections.
func (m *DailyReviewModel) loadTasks() {
	m.resetDailyReviewSections()

	thresholds := newDailyReviewThresholds(m.config, time.Now())
	for _, project := range m.config.GetActiveProjects() {
		if err := storage.LoadProject(project); err != nil {
			continue
		}
		m.loadDailyReviewProject(project, thresholds)
	}

	m.sortSections()
	m.calculateNavigation()
}

type dailyReviewThresholds struct {
	today              time.Time
	dueSoonDays        int
	futureDays         int
	oldDays            int
	completedHours     int
	escalationDays     int
	dueSoonEnd         time.Time
	escalationEnd      time.Time
	futureStart        time.Time
	oldThreshold       time.Time
	completedThreshold time.Time
}

func (m *DailyReviewModel) resetDailyReviewSections() {
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
}

func newDailyReviewThresholds(cfg *config.Config, now time.Time) dailyReviewThresholds {
	today := util.StartOfDay(now)
	dueSoonDays := cfg.GetReviewDueSoonDays()
	futureDays := cfg.GetReviewFutureDays()
	oldDays := cfg.GetReviewOldDays()
	completedHours := cfg.GetReviewCompletedHours()
	escalationDays := cfg.GetPriorityEscalationDays()

	return dailyReviewThresholds{
		today:              today,
		dueSoonDays:        dueSoonDays,
		futureDays:         futureDays,
		oldDays:            oldDays,
		completedHours:     completedHours,
		escalationDays:     escalationDays,
		dueSoonEnd:         today.AddDate(0, 0, dueSoonDays+1),
		escalationEnd:      today.AddDate(0, 0, escalationDays+1),
		futureStart:        today.AddDate(0, 0, futureDays),
		oldThreshold:       now.AddDate(0, 0, -oldDays),
		completedThreshold: now.Add(-time.Duration(completedHours) * time.Hour),
	}
}

func (m *DailyReviewModel) loadDailyReviewProject(project *models.Project, thresholds dailyReviewThresholds) {
	for _, task := range project.Tasks {
		if m.shouldSkipDailyReviewTask(task, project) {
			continue
		}
		m.appendDailyReviewTask(CrossProjectTask{Task: task, Project: project}, project, thresholds)
	}
}

func (m *DailyReviewModel) shouldSkipDailyReviewTask(task *models.Task, project *models.Project) bool {
	if task == nil {
		return true
	}
	if !hasActiveCommonFilters(&m.filter) {
		return false
	}
	return !passesCommonFilters(task, &m.filter, project.ID, m.config)
}

func (m *DailyReviewModel) appendDailyReviewTask(dt CrossProjectTask, project *models.Project, thresholds dailyReviewThresholds) {
	if m.appendCompletedDailyReviewTask(dt, thresholds) {
		return
	}
	if dt.Task.Status.IsComplete() || dt.Task.IsSnoozed() {
		return
	}
	if m.appendStandbyOrReviewTask(dt) {
		return
	}
	if m.appendDatedDailyReviewTask(dt, thresholds) {
		return
	}
	if m.appendBlockedOrProgressTask(dt, project) {
		return
	}
	m.appendOldDailyReviewTask(dt, thresholds)
}

func (m *DailyReviewModel) appendCompletedDailyReviewTask(dt CrossProjectTask, thresholds dailyReviewThresholds) bool {
	task := dt.Task
	if !task.Status.IsComplete() || task.CompletedAt == nil {
		return false
	}
	if thresholds.completedHours > 0 && task.CompletedAt.After(thresholds.completedThreshold) {
		m.completed = append(m.completed, dt)
	}
	return true
}

func (m *DailyReviewModel) appendStandbyOrReviewTask(dt CrossProjectTask) bool {
	task := dt.Task
	if task.Section == models.SectionStandby || task.Status == models.StatusStandby {
		m.inStandby = append(m.inStandby, dt)
		return true
	}
	if task.Section == models.SectionReview || task.Status == models.StatusReview {
		m.inReview = append(m.inReview, dt)
		return true
	}
	return false
}

func (m *DailyReviewModel) appendDatedDailyReviewTask(dt CrossProjectTask, thresholds dailyReviewThresholds) bool {
	task := dt.Task
	if task.DueDate == nil {
		return false
	}

	dueDay := util.StartOfDay(*task.DueDate)
	if thresholds.escalationDays > 0 && task.Priority >= 3 && dueDay.Before(thresholds.escalationEnd) {
		dt.SuggestEscalation = true
	}

	switch {
	case dueDay.Before(thresholds.today):
		m.overdue = append(m.overdue, dt)
	case dueDay.Equal(thresholds.today):
		m.dueToday = append(m.dueToday, dt)
	case thresholds.dueSoonDays > 0 && dueDay.Before(thresholds.dueSoonEnd):
		m.dueSoon = append(m.dueSoon, dt)
	case thresholds.futureDays > 0 && !dueDay.Before(thresholds.futureStart):
		m.future = append(m.future, dt)
	default:
		m.dueLater = append(m.dueLater, dt)
	}
	return true
}

func (m *DailyReviewModel) appendBlockedOrProgressTask(dt CrossProjectTask, project *models.Project) bool {
	task := dt.Task
	if task.Status == models.StatusBlocked || project.HasIncompleteDependencies(task) {
		m.blocked = append(m.blocked, dt)
		return true
	}
	if task.Status == models.StatusInProgress {
		m.inProgress = append(m.inProgress, dt)
		return true
	}
	return false
}

func (m *DailyReviewModel) appendOldDailyReviewTask(dt CrossProjectTask, thresholds dailyReviewThresholds) {
	effectiveUpdate := dt.Task.UpdatedAt
	if effectiveUpdate.IsZero() {
		effectiveUpdate = dt.Task.CreatedAt
	}
	if thresholds.oldDays > 0 && effectiveUpdate.Before(thresholds.oldThreshold) {
		m.old = append(m.old, dt)
	}
}

type crossTaskLess func(a, b CrossProjectTask) bool

func byProjectDisplayOrderThen(less crossTaskLess) crossTaskLess {
	return func(a, b CrossProjectTask) bool {
		if c := compareProjectsByDisplayOrder(a.Project, b.Project); c != 0 {
			return c < 0
		}
		return less(a, b)
	}
}

func sortDailyReviewSection(tasks []CrossProjectTask, less crossTaskLess) {
	less = byProjectDisplayOrderThen(less)
	sort.Slice(tasks, func(i, j int) bool { return less(tasks[i], tasks[j]) })
}

// crossTaskLess comparators for recurring sort patterns on CrossProjectTask slices.
func byOverdueDueDateThenID(a, b CrossProjectTask) bool {
	if !a.Task.DueDate.Equal(*b.Task.DueDate) {
		return a.Task.DueDate.Before(*b.Task.DueDate)
	}
	return a.Task.ID < b.Task.ID
}

func byDueDateThenPriority(a, b CrossProjectTask) bool {
	if !a.Task.DueDate.Equal(*b.Task.DueDate) {
		return a.Task.DueDate.Before(*b.Task.DueDate)
	}
	if a.Task.Priority != b.Task.Priority {
		return a.Task.Priority < b.Task.Priority
	}
	return a.Task.ID < b.Task.ID
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
	if !a.Task.UpdatedAt.Equal(b.Task.UpdatedAt) {
		return a.Task.UpdatedAt.After(b.Task.UpdatedAt)
	}
	return a.Task.ID < b.Task.ID
}

func byEffectiveUpdatedAtThenID(a, b CrossProjectTask) bool {
	ti := a.Task.UpdatedAt
	if ti.IsZero() {
		ti = a.Task.CreatedAt
	}
	tj := b.Task.UpdatedAt
	if tj.IsZero() {
		tj = b.Task.CreatedAt
	}
	if !ti.Equal(tj) {
		return ti.Before(tj)
	}
	return a.Task.ID < b.Task.ID
}

func byCompletedAtThenID(a, b CrossProjectTask) bool {
	if a.Task.CompletedAt == nil {
		return false
	}
	if b.Task.CompletedAt == nil {
		return true
	}
	if !a.Task.CompletedAt.Equal(*b.Task.CompletedAt) {
		return a.Task.CompletedAt.After(*b.Task.CompletedAt)
	}
	return a.Task.ID < b.Task.ID
}

// sortSections sorts each section by appropriate criteria.
func (m *DailyReviewModel) sortSections() {
	// Overdue: oldest first (most urgent), then by ID for stability
	sortDailyReviewSection(m.overdue, byOverdueDueDateThenID)

	sortDailyReviewSection(m.dueToday, byPriorityThenID)
	sortDailyReviewSection(m.dueSoon, byDueDateThenPriority)
	sortDailyReviewSection(m.dueLater, byDueDateThenPriority)
	sortDailyReviewSection(m.future, byDueDateThenPriority)
	sortDailyReviewSection(m.inProgress, byPriorityThenUpdatedAt)
	sortDailyReviewSection(m.blocked, byPriorityThenID)
	sortDailyReviewSection(m.inReview, byPriorityThenUpdatedAt)
	sortDailyReviewSection(m.inStandby, byPriorityThenUpdatedAt)

	// Old: by UpdatedAt (oldest first), falling back to CreatedAt when zero
	sortDailyReviewSection(m.old, byEffectiveUpdatedAtThenID)

	// Completed: most recent first
	sortDailyReviewSection(m.completed, byCompletedAtThenID)
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
