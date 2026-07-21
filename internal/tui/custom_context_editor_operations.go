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

	renamed := m.editMode && !strings.EqualFold(name, m.originalName)
	if err := commitMachineToRuntime(m.runtime, func(machine *config.MachineState) {
		if renamed {
			machine.DeleteCustomContext(m.originalName)
		}
		machine.SetCustomContext(name, projectIDs)
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
