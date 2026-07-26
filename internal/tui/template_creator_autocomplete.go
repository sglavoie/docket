package tui

import (
	"sort"
	"strings"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

// buildCaches gathers canonical states and project tags for autocomplete.
func (m *TaskTemplateCreatorModel) buildCaches() {
	tagSet := make(map[string]struct{})

	for _, proj := range m.runtime.AllLoadableProjects() {
		if err := storage.LoadProject(proj); err == nil {
			for _, tag := range CollectTagNamesForProject(proj) {
				tagSet[tag] = struct{}{}
			}
		}
	}

	m.stateCache = make([]string, 0, len(models.AllWorkflowStates()))
	for _, state := range models.AllWorkflowStates() {
		m.stateCache = append(m.stateCache, state.String())
	}

	m.tagCache = make([]string, 0, len(tagSet))
	for tag := range tagSet {
		m.tagCache = append(m.tagCache, tag)
	}
	sort.Strings(m.tagCache)
}

// updateStateSuggestions filters canonical states based on current input.
func (m *TaskTemplateCreatorModel) updateStateSuggestions() {
	query := strings.ToLower(strings.TrimSpace(m.stateInput.Value()))
	if query == "" {
		m.stateSuggestions = nil
		m.showSuggestions = false
		return
	}

	m.stateSuggestions = nil
	for _, state := range m.stateCache {
		if strings.Contains(strings.ToLower(state), query) && strings.ToLower(state) != query {
			m.stateSuggestions = append(m.stateSuggestions, state)
		}
	}
	m.showSuggestions = len(m.stateSuggestions) > 0
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

// acceptStateSuggestion accepts the currently selected state suggestion.
func (m *TaskTemplateCreatorModel) acceptStateSuggestion() {
	if m.suggestionCursor < len(m.stateSuggestions) {
		m.stateInput.SetValue(m.stateSuggestions[m.suggestionCursor])
		m.stateInput.SetCursor(len(m.stateInput.Value()))
	}
	m.showSuggestions = false
	m.stateSuggestions = nil
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
	m.stateSuggestions = nil
	m.tagSuggestions = nil
	m.suggestionCursor = 0
}

// getCurrentSuggestions returns the current suggestion list based on focused field.
func (m *TaskTemplateCreatorModel) getCurrentSuggestions() []string {
	section, _ := m.getFocusSection()
	switch section {
	case "state":
		return m.stateSuggestions
	case "tags":
		return m.tagSuggestions
	}
	return nil
}

// showAllStateSuggestions populates stateSuggestions with all canonical states.
func (m *TaskTemplateCreatorModel) showAllStateSuggestions() {
	currentValue := strings.ToLower(strings.TrimSpace(m.stateInput.Value()))
	m.stateSuggestions = nil
	for _, state := range m.stateCache {
		if strings.ToLower(state) != currentValue {
			m.stateSuggestions = append(m.stateSuggestions, state)
		}
	}
	if len(m.stateSuggestions) > 0 {
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
