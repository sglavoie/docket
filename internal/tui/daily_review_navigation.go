package tui

import tea "github.com/charmbracelet/bubbletea"

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
		{SectionOld, m.old},
		{SectionCompleted, m.completed},
	}

	for _, s := range sections {
		if len(s.tasks) > 0 {
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

// getCurrentSectionIndex returns the index of the section containing the cursor.
func (m *DailyReviewModel) getCurrentSectionIndex() int {
	for i := len(m.sectionStarts) - 1; i >= 0; i-- {
		if m.cursor >= m.sectionStarts[i] {
			return i
		}
	}
	return 0
}

// goToNextSection moves cursor to next section header (with wrap).
func (m *DailyReviewModel) goToNextSection() (tea.Model, tea.Cmd) {
	if len(m.sectionOrder) <= 1 {
		return m, nil
	}

	// Find current section index
	currentSectionIdx := m.getCurrentSectionIndex()

	// Move to next section (with wrap)
	nextSectionIdx := (currentSectionIdx + 1) % len(m.sectionOrder)
	m.cursor = m.sectionStarts[nextSectionIdx]

	return m, nil
}

// goToPrevSection moves cursor to previous section header (with wrap).
func (m *DailyReviewModel) goToPrevSection() (tea.Model, tea.Cmd) {
	if len(m.sectionOrder) <= 1 {
		return m, nil
	}

	// Find current section index
	currentSectionIdx := m.getCurrentSectionIndex()

	// Move to previous section (with wrap)
	prevSectionIdx := currentSectionIdx - 1
	if prevSectionIdx < 0 {
		prevSectionIdx = len(m.sectionOrder) - 1
	}
	m.cursor = m.sectionStarts[prevSectionIdx]

	return m, nil
}

// clampCursor ensures cursor is within valid range.
func (m *DailyReviewModel) clampCursor() {
	if m.cursor >= m.totalTasks {
		m.cursor = m.totalTasks - 1
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
}
