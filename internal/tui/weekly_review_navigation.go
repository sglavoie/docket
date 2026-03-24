package tui

import tea "charm.land/bubbletea/v2"

// weeklyReviewTabNames are the tab labels for the weekly review view.
var weeklyReviewTabNames = []string{"Projects", "Tasks", "Planning"}

// weeklyReviewTabSections maps each tab index to the sections it contains.
var weeklyReviewTabSections = [][]WeeklyReviewSection{
	{WeeklySectionStaleProjects, WeeklySectionInboxBacklog, WeeklySectionBlockedWork, WeeklySectionOvercommitted},
	{WeeklySectionStaleTasks, WeeklySectionPriorityMismatch},
	{WeeklySectionUpcomingWeek, WeeklySectionRecurringOverview},
}

// calculateNavigation builds navigation indices including section headers.
func (m *WeeklyReviewModel) calculateNavigation() {
	entries := []SectionEntry{
		{int(WeeklySectionStaleProjects), len(m.staleProjects)},
		{int(WeeklySectionInboxBacklog), len(m.inboxBacklog)},
		{int(WeeklySectionBlockedWork), len(m.blockedWork)},
		{int(WeeklySectionOvercommitted), len(m.overcommitted)},
		{int(WeeklySectionStaleTasks), len(m.staleTasks)},
		{int(WeeklySectionPriorityMismatch), len(m.priorityMismatch)},
		{int(WeeklySectionUpcomingWeek), len(m.upcomingWeek)},
		{int(WeeklySectionRecurringOverview), len(m.recurringOverview)},
	}

	var visibleSet map[int]bool
	if m.tabs.Active >= 0 && m.tabs.Active < len(weeklyReviewTabSections) {
		visibleSet = make(map[int]bool)
		for _, s := range weeklyReviewTabSections[m.tabs.Active] {
			visibleSet[int(s)] = true
		}
	}

	var focusedID *int
	if m.focusedSection != nil {
		id := int(*m.focusedSection)
		focusedID = &id
	}

	result := CalculateSectionNav(entries, m.collapsedSections, visibleSet, focusedID)

	m.sectionOrder = make([]WeeklyReviewSection, len(result.SectionOrder))
	for i, id := range result.SectionOrder {
		m.sectionOrder[i] = WeeklyReviewSection(id)
	}
	m.sectionStarts = result.SectionStarts
	m.totalItems = result.TotalItems
}

// getSectionItemCount returns the number of items in a section.
func (m *WeeklyReviewModel) getSectionItemCount(section WeeklyReviewSection) int {
	switch section {
	case WeeklySectionStaleProjects:
		return len(m.staleProjects)
	case WeeklySectionInboxBacklog:
		return len(m.inboxBacklog)
	case WeeklySectionBlockedWork:
		return len(m.blockedWork)
	case WeeklySectionOvercommitted:
		return len(m.overcommitted)
	case WeeklySectionStaleTasks:
		return len(m.staleTasks)
	case WeeklySectionPriorityMismatch:
		return len(m.priorityMismatch)
	case WeeklySectionUpcomingWeek:
		return len(m.upcomingWeek)
	case WeeklySectionRecurringOverview:
		return len(m.recurringOverview)
	}
	return 0
}

// getProjectAtCursor returns the project at the current cursor position, or nil.
func (m *WeeklyReviewModel) getProjectAtCursor() *WeeklyReviewProject {
	if m.totalItems == 0 || len(m.sectionOrder) == 0 {
		return nil
	}

	pos := 0
	for _, section := range m.sectionOrder {
		if m.cursor == pos {
			return nil // On header
		}
		pos++

		if m.collapsedSections[int(section)] {
			continue
		}

		count := m.getSectionItemCount(section)
		if m.cursor >= pos && m.cursor < pos+count {
			idx := m.cursor - pos
			switch section {
			case WeeklySectionStaleProjects:
				if idx < len(m.staleProjects) {
					return &m.staleProjects[idx]
				}
			case WeeklySectionInboxBacklog:
				if idx < len(m.inboxBacklog) {
					return &m.inboxBacklog[idx]
				}
			case WeeklySectionBlockedWork:
				if idx < len(m.blockedWork) {
					return &m.blockedWork[idx]
				}
			case WeeklySectionOvercommitted:
				if idx < len(m.overcommitted) {
					return &m.overcommitted[idx]
				}
			default:
			}
			return nil // Task sections don't return projects
		}

		pos += count
	}

	return nil
}

// getTaskAtCursor returns the task at the current cursor position, or nil.
func (m *WeeklyReviewModel) getTaskAtCursor() *WeeklyReviewTask {
	if m.totalItems == 0 || len(m.sectionOrder) == 0 {
		return nil
	}

	pos := 0
	for _, section := range m.sectionOrder {
		if m.cursor == pos {
			return nil // On header
		}
		pos++

		if m.collapsedSections[int(section)] {
			continue
		}

		count := m.getSectionItemCount(section)
		if m.cursor >= pos && m.cursor < pos+count {
			idx := m.cursor - pos
			switch section {
			case WeeklySectionStaleTasks:
				if idx < len(m.staleTasks) {
					return &m.staleTasks[idx]
				}
			case WeeklySectionPriorityMismatch:
				if idx < len(m.priorityMismatch) {
					return &m.priorityMismatch[idx]
				}
			case WeeklySectionUpcomingWeek:
				if idx < len(m.upcomingWeek) {
					return &m.upcomingWeek[idx]
				}
			case WeeklySectionRecurringOverview:
				if idx < len(m.recurringOverview) {
					return &m.recurringOverview[idx]
				}
			default:
			}
			return nil // Project sections don't return tasks
		}

		pos += count
	}

	return nil
}

// getCurrentSectionIndex returns the index of the section containing the cursor.
func (m *WeeklyReviewModel) getCurrentSectionIndex() int {
	return getCurrentSectionIdx(m.cursor, m.sectionStarts)
}

// goToNextSection moves cursor to next section header (with wrap).
func (m *WeeklyReviewModel) goToNextSection() (tea.Model, tea.Cmd) {
	return m, m.SectionGoToNext(m.sectionStarts)
}

// goToPrevSection moves cursor to previous section header (with wrap).
func (m *WeeklyReviewModel) goToPrevSection() (tea.Model, tea.Cmd) {
	return m, m.SectionGoToPrev(m.sectionStarts)
}

// goToNextUnreviewed jumps to the next unreviewed section with items, crossing tabs.
func (m *WeeklyReviewModel) goToNextUnreviewed() (tea.Model, tea.Cmd) {
	allSections := []WeeklyReviewSection{
		WeeklySectionStaleProjects, WeeklySectionInboxBacklog,
		WeeklySectionBlockedWork, WeeklySectionOvercommitted,
		WeeklySectionStaleTasks, WeeklySectionPriorityMismatch,
		WeeklySectionUpcomingWeek, WeeklySectionRecurringOverview,
	}
	for _, section := range allSections {
		if m.reviewedSections[section] {
			continue
		}
		if m.getSectionItemCount(section) == 0 {
			continue
		}
		// Find which tab contains this section and switch to it
		for tabIdx, tabSections := range weeklyReviewTabSections {
			for _, ts := range tabSections {
				if ts == section {
					m.tabs.Active = tabIdx
					m.focusedSection = nil
					m.scrollOffset = 0
					m.calculateNavigation()
					// Find the section's start position in current navigation
					for i, s := range m.sectionOrder {
						if s == section {
							m.cursor = m.sectionStarts[i]
							return m, nil
						}
					}
					return m, nil
				}
			}
		}
	}
	m.setStatus("All sections reviewed")
	return m, nil
}

// clampCursor ensures cursor is within valid range.
func (m *WeeklyReviewModel) clampCursor() {
	m.ClampCursorTo(m.totalItems)
}
