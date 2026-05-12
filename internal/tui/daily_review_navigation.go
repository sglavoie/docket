package tui

import tea "charm.land/bubbletea/v2"

// dailyReviewTabNames are the tab labels for the daily review view.
var dailyReviewTabNames = []string{"Due", "Done", "Active", "In Review", "Stale"}

// dailyReviewTabSections maps each tab index to the sections it contains.
var dailyReviewTabSections = [][]DailyReviewSection{
	{SectionOverdue, SectionDueToday, SectionDueSoon, SectionDueLater, SectionFuture}, // Due
	{SectionCompleted},                  // Done
	{SectionInProgress, SectionBlocked}, // Active
	{SectionInReview, SectionOnStandby}, // In Review
	{SectionOld},                        // Stale
}

// calculateNavigation builds navigation indices including section headers.
// Each section header is a navigable item, followed by its tasks (if expanded).
func (m *DailyReviewModel) calculateNavigation() {
	entries := []SectionEntry{
		{int(SectionOverdue), len(m.overdue)},
		{int(SectionDueToday), len(m.dueToday)},
		{int(SectionDueSoon), len(m.dueSoon)},
		{int(SectionDueLater), len(m.dueLater)},
		{int(SectionFuture), len(m.future)},
		{int(SectionInProgress), len(m.inProgress)},
		{int(SectionBlocked), len(m.blocked)},
		{int(SectionInReview), len(m.inReview)},
		{int(SectionOnStandby), len(m.inStandby)},
		{int(SectionOld), len(m.old)},
		{int(SectionCompleted), len(m.completed)},
	}

	var visibleSet map[int]bool
	if m.tabs.Active >= 0 && m.tabs.Active < len(dailyReviewTabSections) {
		visibleSet = make(map[int]bool)
		for _, s := range dailyReviewTabSections[m.tabs.Active] {
			visibleSet[int(s)] = true
		}
	}

	var focusedID *int
	if m.focusedSection != nil {
		id := int(*m.focusedSection)
		focusedID = &id
	}

	result := CalculateSectionNav(entries, m.collapsedSections, visibleSet, focusedID)

	m.sectionOrder = make([]DailyReviewSection, len(result.SectionOrder))
	for i, id := range result.SectionOrder {
		m.sectionOrder[i] = DailyReviewSection(id)
	}
	m.sectionStarts = result.SectionStarts
	m.totalTasks = result.TotalItems
}

// getSectionTasks returns the tasks for a given section.
func (m *DailyReviewModel) getSectionTasks(section DailyReviewSection) []CrossProjectTask {
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
func (m *DailyReviewModel) getTaskAtCursor() *CrossProjectTask {
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
	return m, m.SectionGoToNext(m.sectionStarts)
}

// goToPrevSection moves cursor to previous section header (with wrap).
func (m *DailyReviewModel) goToPrevSection() (tea.Model, tea.Cmd) {
	return m, m.SectionGoToPrev(m.sectionStarts)
}

// clampCursor ensures cursor is within valid range.
func (m *DailyReviewModel) clampCursor() {
	m.ClampCursorTo(m.totalTasks)
}
