package tui

import tea "github.com/charmbracelet/bubbletea"

// dailyReviewTabNames are the tab labels for the daily review view.
var dailyReviewTabNames = []string{"Due", "Active", "In Review", "Stale", "Done"}

// dailyReviewTabSections maps each tab index to the sections it contains.
var dailyReviewTabSections = [][]DailyReviewSection{
	{SectionOverdue, SectionDueToday, SectionDueSoon, SectionDueLater, SectionFuture}, // Due
	{SectionInProgress, SectionBlocked},                                               // Active
	{SectionInReview, SectionOnStandby},                                               // In Review
	{SectionOld},                                                                      // Stale
	{SectionCompleted},                                                                // Done
}

// sectionInTab returns true if the given section belongs to the active tab.
func (m *DailyReviewModel) sectionInTab(section DailyReviewSection) bool {
	if m.activeTab < 0 || m.activeTab >= len(dailyReviewTabSections) {
		return true
	}
	for _, s := range dailyReviewTabSections[m.activeTab] {
		if s == section {
			return true
		}
	}
	return false
}

// calculateNavigation builds navigation indices including section headers.
// Each section header is a navigable item, followed by its tasks (if expanded).
func (m *DailyReviewModel) calculateNavigation() {
	m.sectionOrder = nil
	m.sectionStarts = nil
	m.totalTasks = 0

	sections := []struct {
		section DailyReviewSection
		tasks   []DailyReviewTask
	}{
		{SectionOverdue, m.overdue},
		{SectionDueToday, m.dueToday},
		{SectionDueSoon, m.dueSoon},
		{SectionDueLater, m.dueLater},
		{SectionFuture, m.future},
		{SectionInProgress, m.inProgress},
		{SectionBlocked, m.blocked},
		{SectionInReview, m.inReview},
		{SectionOnStandby, m.inStandby},
		{SectionOld, m.old},
		{SectionCompleted, m.completed},
	}

	for _, s := range sections {
		if len(s.tasks) > 0 {
			// Filter by tab
			if !m.sectionInTab(s.section) {
				continue
			}
			// If focused on a section, only show that section
			if m.focusedSection != nil && *m.focusedSection != s.section {
				continue
			}
			m.sectionOrder = append(m.sectionOrder, s.section)
			m.sectionStarts = append(m.sectionStarts, m.totalTasks)
			m.totalTasks++ // Count header as navigable item
			if !m.collapsedSections[int(s.section)] {
				m.totalTasks += len(s.tasks)
			}
		}
	}
}

// getSectionTasks returns the tasks for a given section.
func (m *DailyReviewModel) getSectionTasks(section DailyReviewSection) []DailyReviewTask {
	switch section {
	case SectionOverdue:
		return m.overdue
	case SectionDueToday:
		return m.dueToday
	case SectionDueSoon:
		return m.dueSoon
	case SectionDueLater:
		return m.dueLater
	case SectionFuture:
		return m.future
	case SectionInProgress:
		return m.inProgress
	case SectionBlocked:
		return m.blocked
	case SectionInReview:
		return m.inReview
	case SectionOnStandby:
		return m.inStandby
	case SectionOld:
		return m.old
	case SectionCompleted:
		return m.completed
	}
	return nil
}

// getTaskAtCursor returns the task at the current cursor position.
// Returns nil if cursor is on a section header.
func (m *DailyReviewModel) getTaskAtCursor() *DailyReviewTask {
	if m.totalTasks == 0 || len(m.sectionOrder) == 0 {
		return nil
	}

	pos := 0
	for _, section := range m.sectionOrder {
		// Check if cursor is on section header
		if m.cursor == pos {
			return nil // On header, not a task
		}
		pos++ // Move past header

		if m.collapsedSections[int(section)] {
			continue // No tasks to check
		}

		tasks := m.getSectionTasks(section)
		sectionEnd := pos + len(tasks)

		if m.cursor >= pos && m.cursor < sectionEnd {
			idx := m.cursor - pos
			if idx >= 0 && idx < len(tasks) {
				return &tasks[idx]
			}
		}

		pos = sectionEnd
	}

	return nil
}

// goToNextSection moves cursor to next section header (with wrap).
func (m *DailyReviewModel) goToNextSection() (tea.Model, tea.Cmd) {
	if pos := navigateToNextSection(m.cursor, m.sectionStarts); pos >= 0 {
		m.cursor = pos
	}
	return m, nil
}

// goToPrevSection moves cursor to previous section header (with wrap).
func (m *DailyReviewModel) goToPrevSection() (tea.Model, tea.Cmd) {
	if pos := navigateToPrevSection(m.cursor, m.sectionStarts); pos >= 0 {
		m.cursor = pos
	}
	return m, nil
}

// clampCursor ensures cursor is within valid range.
func (m *DailyReviewModel) clampCursor() {
	m.ClampCursorTo(m.totalTasks)
}
