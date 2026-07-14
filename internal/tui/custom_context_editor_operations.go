package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/config"
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
	// Custom contexts and their cycle lists are machine-local navigation state,
	// committed here through a checked machine commit (not a whole-config save).
	name := strings.TrimSpace(m.nameInput.Value())
	if name == "" {
		return func() tea.Msg { return ErrorMsg{Err: fmt.Errorf("custom context name cannot be empty")} }
	}

	machine := m.runtime.Machine()

	// Check for duplicate name (case-insensitive) when creating new
	if !m.editMode && machine.CustomContextExists(name) {
		return func() tea.Msg { return ErrorMsg{Err: fmt.Errorf("custom context '%s' already exists", name)} }
	}

	// When editing, check if name changed and new name already exists
	if m.editMode && !strings.EqualFold(name, m.originalName) && machine.CustomContextExists(name) {
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

	// Compute the cycle IDs for this context up front so the commit closure is a
	// pure application of the already-decided mutations.
	contextKey := "+" + name
	var newCycleIDs []string
	if m.editMode {
		// Preserve existing cycle selections, adjusting for added/removed projects
		existingCycleIDs := machine.GetCycleProjectIDsForContext(contextKey)
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
		for _, id := range existingCycleIDs {
			if newProjectSet[id] {
				newCycleIDs = append(newCycleIDs, id)
			}
		}
		// Add newly added projects (not in old context) to cycle
		for _, id := range projectIDs {
			if !oldProjectSet[id] {
				newCycleIDs = append(newCycleIDs, id)
			}
		}
	} else {
		// New context: select all projects by default
		newCycleIDs = projectIDs
	}

	renamed := m.editMode && !strings.EqualFold(name, m.originalName)
	if err := commitMachineToRuntime(m.runtime, func(machine *config.MachineState) {
		// Delete old name if we renamed, migrating cycle projects to the new key.
		if renamed {
			machine.DeleteCustomContext(m.originalName)
			oldKey := "+" + m.originalName
			if oldCycleIDs := machine.GetCycleProjectIDsForContext(oldKey); len(oldCycleIDs) > 0 {
				machine.SetCycleProjectsForContext(oldKey, nil)
			}
		}
		machine.SetCustomContext(name, projectIDs)
		machine.SetCycleProjectsForContext(contextKey, newCycleIDs)
	}); err != nil {
		return func() tea.Msg { return ErrorMsg{Err: err} }
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
