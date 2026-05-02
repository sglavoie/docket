package tui

import "strings"

func (m *TutorModel) filterTopics() {
	source := m.topics
	if m.activeTab > 0 && m.activeTab <= len(m.categories) {
		cat := m.categories[m.activeTab-1]
		source = nil
		for _, t := range m.topics {
			if t.Category == cat {
				source = append(source, t)
			}
		}
	}

	if m.searchQuery == "" {
		m.filteredTopics = source
		return
	}

	query := strings.ToLower(m.searchQuery)
	var filtered []TutorTopic
	for _, t := range source {
		if strings.Contains(strings.ToLower(t.Title), query) ||
			strings.Contains(strings.ToLower(t.Content), query) ||
			strings.Contains(strings.ToLower(t.Category), query) {
			filtered = append(filtered, t)
		}
	}
	m.filteredTopics = filtered
}

func (m *TutorModel) updateViewport() {
	if !m.ready {
		return
	}
	m.viewport.SetContent(m.renderTopicContent())
	m.viewport.GotoTop()
}

// topicLineIndex returns the line number (0-based) of the given topic index
// in the rendered list, accounting for category headers and blank separators.
func (m *TutorModel) topicLineIndex(topicIdx int) int {
	line := 0
	lastCat := ""
	for i, t := range m.filteredTopics {
		if t.Category != lastCat {
			if lastCat != "" {
				line++ // blank separator between categories
			}
			line++ // category header
			lastCat = t.Category
		}
		if i == topicIdx {
			return line
		}
		line++
	}
	return line
}

// listViewHeight returns the visible height for the topic list pane.
func (m *TutorModel) listViewHeight() int {
	return m.viewport.Height()
}

// ensureSelectedVisible adjusts listOffset so the selected item is visible.
// When the selected item is the first in its category, the category header
// (and any blank separator above it) is also kept visible.
func (m *TutorModel) ensureSelectedVisible() {
	if len(m.filteredTopics) == 0 {
		m.listOffset = 0
		return
	}
	h := m.listViewHeight()
	line := m.topicLineIndex(m.selectedIndex)

	// If this is the first topic in its category, include the header above it.
	headerLine := line
	if m.selectedIndex == 0 || m.filteredTopics[m.selectedIndex].Category != m.filteredTopics[m.selectedIndex-1].Category {
		// The category header is the line right before this topic.
		// For non-first categories there's also a blank separator line before the header.
		if m.selectedIndex > 0 {
			headerLine = line - 2 // blank + header
		} else {
			headerLine = line - 1 // just header
		}
		if headerLine < 0 {
			headerLine = 0
		}
	}

	if headerLine < m.listOffset {
		m.listOffset = headerLine
	} else if line >= m.listOffset+h {
		m.listOffset = line - h + 1
	}
}
