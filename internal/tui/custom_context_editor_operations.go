package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
)

func (m *CustomContextEditorModel) filterProjects() {
	query := strings.ToLower(m.SearchQuery())
	if query == "" {
		m.filteredProjects = m.projects
	} else {
		m.filteredProjects = nil
		for _, p := range m.projects {
			if strings.Contains(strings.ToLower(p.Name), query) ||
				strings.Contains(strings.ToLower(p.Context), query) {
				m.filteredProjects = append(m.filteredProjects, p)
			}
		}
	}
	m.cursor = 0
	m.scrollOffset = 0
}

func (m *CustomContextEditorModel) save() tea.Cmd {
	// All mutations run synchronously to avoid data races with View().
	// Config is persisted by the CustomContextSavedMsg handler.
	name := strings.TrimSpace(m.nameInput.Value())
	if name == "" {
		return func() tea.Msg { return ErrorMsg{Err: fmt.Errorf("custom context name cannot be empty")} }
	}

	// Check for duplicate name (case-insensitive) when creating new
	if !m.editMode && m.config.CustomContextExists(name) {
		return func() tea.Msg { return ErrorMsg{Err: fmt.Errorf("custom context '%s' already exists", name)} }
	}

	// When editing, check if name changed and new name already exists
	if m.editMode && !strings.EqualFold(name, m.originalName) && m.config.CustomContextExists(name) {
		return func() tea.Msg { return ErrorMsg{Err: fmt.Errorf("custom context '%s' already exists", name)} }
	}

	// Collect selected project IDs
	var projectIDs []string
	for _, p := range m.projects {
		if m.selectedIDs[p.ID] {
			projectIDs = append(projectIDs, p.ID)
		}
	}

	if len(projectIDs) == 0 {
		return func() tea.Msg { return ErrorMsg{Err: fmt.Errorf("select at least one project")} }
	}

	// Delete old name if we renamed
	if m.editMode && !strings.EqualFold(name, m.originalName) {
		m.config.DeleteCustomContext(m.originalName)
		// Also migrate cycle projects to new key
		oldKey := "+" + m.originalName
		newKey := "+" + name
		if oldCycleIDs := m.config.GetCycleProjectIDsForContext(oldKey); len(oldCycleIDs) > 0 {
			m.config.SetCycleProjectsForContext(newKey, oldCycleIDs)
			m.config.SetCycleProjectsForContext(oldKey, nil)
		}
	}

	// Save custom context
	m.config.SetCustomContext(name, projectIDs)

	// Update cycle projects for this context
	contextKey := "+" + name
	if m.editMode {
		// Preserve existing cycle selections, adjusting for added/removed projects
		existingCycleIDs := m.config.GetCycleProjectIDsForContext(contextKey)
		newProjectSet := make(map[string]bool, len(projectIDs))
		for _, id := range projectIDs {
			newProjectSet[id] = true
		}
		oldProjectSet := make(map[string]bool, len(m.initialSelectedIDs))
		for id, sel := range m.initialSelectedIDs {
			if sel {
				oldProjectSet[id] = true
			}
		}
		// Keep existing cycle IDs that are still in the context
		var updatedCycleIDs []string
		for _, id := range existingCycleIDs {
			if newProjectSet[id] {
				updatedCycleIDs = append(updatedCycleIDs, id)
			}
		}
		// Add newly added projects (not in old context) to cycle
		for _, id := range projectIDs {
			if !oldProjectSet[id] {
				updatedCycleIDs = append(updatedCycleIDs, id)
			}
		}
		m.config.SetCycleProjectsForContext(contextKey, updatedCycleIDs)
	} else {
		// New context: select all projects by default
		m.config.SetCycleProjectsForContext(contextKey, projectIDs)
	}

	isNew := !m.editMode
	count := len(projectIDs)
	return func() tea.Msg {
		return CustomContextSavedMsg{
			Name:         name,
			IsNew:        isNew,
			ProjectCount: count,
		}
	}
}
