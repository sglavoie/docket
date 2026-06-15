package tui

import (
	"sort"
	"strings"

	"github.com/sglavoie/tt/internal/storage"
)

// buildCaches gathers sections and tags from all projects for autocomplete.
func (m *TaskTemplateCreatorModel) buildCaches() {
	sectionSet := make(map[string]struct{})
	tagSet := make(map[string]struct{})

	for _, proj := range m.config.Projects {
		if err := storage.LoadProject(proj); err == nil {
			for _, section := range proj.Sections {
				sectionSet[section] = struct{}{}
			}
			for _, tag := range CollectTagNamesForProject(proj) {
				tagSet[tag] = struct{}{}
			}
		}
	}

	m.sectionCache = make([]string, 0, len(sectionSet))
	for section := range sectionSet {
		m.sectionCache = append(m.sectionCache, section)
	}
	sort.Strings(m.sectionCache)

	m.tagCache = make([]string, 0, len(tagSet))
	for tag := range tagSet {
		m.tagCache = append(m.tagCache, tag)
	}
	sort.Strings(m.tagCache)
}

// updateSectionSuggestions filters section cache based on current input.
func (m *TaskTemplateCreatorModel) updateSectionSuggestions() {
	query := strings.ToLower(strings.TrimSpace(m.sectionInput.Value()))
	if query == "" {
		m.sectionSuggestions = nil
		m.showSuggestions = false
		return
	}

	m.sectionSuggestions = nil
	for _, section := range m.sectionCache {
		if strings.Contains(strings.ToLower(section), query) && strings.ToLower(section) != query {
			m.sectionSuggestions = append(m.sectionSuggestions, section)
		}
	}
	m.showSuggestions = len(m.sectionSuggestions) > 0
	m.suggestionCursor = 0
}

// updateTagSuggestions filters tag cache based on current tag being typed.
func (m *TaskTemplateCreatorModel) updateTagSuggestions() {
	value := m.tagsInput.Value()
	parts := strings.Split(value, ",")
	currentTag := strings.ToLower(strings.TrimSpace(parts[len(parts)-1]))

	if currentTag == "" {
		m.tagSuggestions = nil
		m.showSuggestions = false
		return
	}

	existingTags := make(map[string]struct{})
	for i := 0; i < len(parts)-1; i++ {
		tag := strings.ToLower(strings.TrimSpace(parts[i]))
		if tag != "" {
			existingTags[tag] = struct{}{}
		}
	}

	m.tagSuggestions = nil
	for _, tag := range m.tagCache {
		tagLower := strings.ToLower(tag)
		if strings.Contains(tagLower, currentTag) && tagLower != currentTag {
			if _, exists := existingTags[tagLower]; !exists {
				m.tagSuggestions = append(m.tagSuggestions, tag)
			}
		}
	}
	m.showSuggestions = len(m.tagSuggestions) > 0
	m.suggestionCursor = 0
}

// acceptSectionSuggestion accepts the currently selected section suggestion.
func (m *TaskTemplateCreatorModel) acceptSectionSuggestion() {
	if m.suggestionCursor < len(m.sectionSuggestions) {
		m.sectionInput.SetValue(m.sectionSuggestions[m.suggestionCursor])
		m.sectionInput.SetCursor(len(m.sectionInput.Value()))
	}
	m.showSuggestions = false
	m.sectionSuggestions = nil
}

// acceptTagSuggestion accepts the currently selected tag suggestion.
func (m *TaskTemplateCreatorModel) acceptTagSuggestion() {
	if m.suggestionCursor < len(m.tagSuggestions) {
		value := m.tagsInput.Value()
		parts := strings.Split(value, ",")
		parts[len(parts)-1] = " " + m.tagSuggestions[m.suggestionCursor]
		if len(parts) == 1 {
			parts[0] = m.tagSuggestions[m.suggestionCursor]
		}
		newValue := strings.Join(parts, ",")
		m.tagsInput.SetValue(newValue)
		m.tagsInput.SetCursor(len(newValue))
	}
	m.showSuggestions = false
	m.tagSuggestions = nil
}

// hideSuggestions hides the suggestions dropdown.
func (m *TaskTemplateCreatorModel) hideSuggestions() {
	m.showSuggestions = false
	m.sectionSuggestions = nil
	m.tagSuggestions = nil
	m.suggestionCursor = 0
}

// getCurrentSuggestions returns the current suggestion list based on focused field.
func (m *TaskTemplateCreatorModel) getCurrentSuggestions() []string {
	section, _ := m.getFocusSection()
	switch section {
	case "section":
		return m.sectionSuggestions
	case "tags":
		return m.tagSuggestions
	}
	return nil
}

// showAllSectionSuggestions populates sectionSuggestions with all cached sections.
func (m *TaskTemplateCreatorModel) showAllSectionSuggestions() {
	currentValue := strings.ToLower(strings.TrimSpace(m.sectionInput.Value()))
	m.sectionSuggestions = nil
	for _, section := range m.sectionCache {
		if strings.ToLower(section) != currentValue {
			m.sectionSuggestions = append(m.sectionSuggestions, section)
		}
	}
	if len(m.sectionSuggestions) > 0 {
		m.showSuggestions = true
		m.suggestionCursor = 0
	}
}

// showAllTagSuggestions populates tagSuggestions with all cached tags (excluding already entered).
func (m *TaskTemplateCreatorModel) showAllTagSuggestions() {
	value := m.tagsInput.Value()
	parts := strings.Split(value, ",")
	existingTags := make(map[string]struct{})
	for _, part := range parts {
		tag := strings.ToLower(strings.TrimSpace(part))
		if tag != "" {
			existingTags[tag] = struct{}{}
		}
	}

	m.tagSuggestions = nil
	for _, tag := range m.tagCache {
		if _, exists := existingTags[strings.ToLower(tag)]; !exists {
			m.tagSuggestions = append(m.tagSuggestions, tag)
		}
	}
	if len(m.tagSuggestions) > 0 {
		m.showSuggestions = true
		m.suggestionCursor = 0
	}
}
