package tui

import "strings"

// filterTemplates filters templates based on the current search query.
func (m *TemplatePickerModel) filterTemplates() {
	query := strings.ToLower(m.searchQuery)
	if query == "" {
		m.filteredTemplates = m.templates
		if m.cursor > len(m.filteredTemplates) {
			m.cursor = len(m.filteredTemplates)
		}
		return
	}

	m.filteredTemplates = nil
	for _, t := range m.templates {
		nameLower := strings.ToLower(t.Name)
		categoryLower := strings.ToLower(t.Category)
		if strings.Contains(nameLower, query) || strings.Contains(categoryLower, query) {
			m.filteredTemplates = append(m.filteredTemplates, t)
		}
	}

	// Clamp cursor
	if m.cursor > len(m.filteredTemplates) {
		m.cursor = len(m.filteredTemplates)
	}
}
