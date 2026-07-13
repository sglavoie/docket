package tui

import (
	"fmt"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
)

// handleFilterPrefix handles the second key after pressing F (filter prefix).
func (m *ProjectTreeModel) handleFilterPrefix(key string) (tea.Model, tea.Cmd) {
	// Clear active saved filter for filter changes (but not for Fv which opens saved filters)
	if key != "v" {
		m.clearActiveSavedFilter()
	}

	// View-specific keys
	switch key {
	case "c":
		if m.viewMode == ViewModeCompleted {
			m.viewMode = ViewModeActive
		} else {
			m.viewMode = ViewModeCompleted
		}
		m.applyFilterChange()
		return m, nil
	case "@":
		return m, func() tea.Msg {
			return ShowSectionFilterSelectorMsg{ProjectTree: m}
		}
	case "d":
		m.pendingPrefix = "Fd"
		return m, nil
	case "r":
		m.filter.Actionable = !m.filter.Actionable
		m.applyFilterChange()
		return m, nil
	case "v":
		m.pendingPrefix = "Fv"
		return m, nil
	case "D":
		return m, func() tea.Msg {
			return ShowColumnPickerMsg{Source: ViewProjectTree, ColumnConfig: m.columnConfig}
		}
	case "x":
		// Override common x to also clear view-specific filters
		clearCommonFilters(&m.filter.CommonFilterState)
		m.filter.Sections = make(map[string]bool)
		m.filter.Actionable = false
		if m.viewMode == ViewModeCompleted {
			m.viewMode = ViewModeActive
		}
		m.ClearSearch()
		m.applyFilterChange()
		return m, nil
	}

	// Common filter prefix keys
	result := handleCommonFilterPrefix(key, &m.filter.CommonFilterState, func() tea.Cmd {
		return buildTagCmdFromProject(m.project, m.filter.Tags, ViewProjectTree, "Filter by Tags", false)
	})
	if cmd := applyCommonFilterResult(result, func(p string) { m.pendingPrefix = p }, m.applyFilterChange); cmd != nil {
		return m, cmd
	}
	return m, nil
}

// handleFilterSubPrefix handles all filter sub-prefix keys (Ft, Fs, Fd, Fa, Fas-Fap, FS).
func (m *ProjectTreeModel) handleFilterSubPrefix(prefix, key string) (tea.Model, tea.Cmd) {
	applyFilterSubPrefix(prefix, key, &m.filter.CommonFilterState, m.config,
		func(p string) { m.pendingPrefix = p }, m.applyFilterChange)
	return m, nil
}

// toggleSort sets the sort field, toggling ascending/descending if already set.
func (m *ProjectTreeModel) toggleSort(field SortField) {
	m.clearActiveSavedFilter()
	m.sort.Toggle(field)
	m.resetViewScrollOffset()
	m.sortNodes()
	m.updateVisibility()
}

// sendTasksToSection moves selected tasks or cursor task to the specified section.
// Use section="" for Active. Captures task states before changes to enable
// event-based workflow triggers.
func (m *ProjectTreeModel) sendTasksToSection(section string, visible []*TreeNode) (bool, tea.Cmd) {
	// Capture task states before changes for event-based workflow triggers
	prevStates := captureTaskStates(m.project.Tasks)

	label := "Move to " + section
	if section == "" {
		label = "Move to Active"
	}

	if m.multiSelectMode && len(m.selectedTasks) > 0 {
		count := 0
		for id := range m.selectedTasks {
			if m.project.MutateTask(id, func(t *models.Task) {
				t.SetSection(section)
			}) {
				m.project.RelocateTaskToEndOfSection(id, section)
				count++
			}
		}
		selectedIDs := make([]string, 0, len(m.selectedTasks))
		for id := range m.selectedTasks {
			selectedIDs = append(selectedIDs, id)
		}
		m.clearSelection()
		m.setUndoContext(label, fmt.Sprintf("%d tasks", count))
		m.undo.TaskIDs = selectedIDs
		return true, m.saveAndRefreshWithPrevStates(prevStates)
	}

	// Single task mode
	if !CursorInBounds(m.cursor, len(visible)) {
		return false, nil
	}
	taskID := visible[m.cursor].Task.ID
	task := m.project.GetTaskByID(taskID)
	if task == nil {
		return false, nil
	}
	m.project.MutateTask(taskID, func(t *models.Task) {
		t.SetSection(section)
	})
	m.project.RelocateTaskToEndOfSection(taskID, section)
	m.setUndoContext(label, task.Title)
	m.undo.TaskIDs = []string{taskID}
	return true, m.saveAndRefreshWithPrevStates(prevStates)
}

// handleSortKeys handles 1-5, 0 - sort operations.
// Returns true if the key was handled.
func (m *ProjectTreeModel) handleSortKeys(key string) (bool, tea.Cmd) {
	handled := handleSortKey(key, &m.sort,
		func() {
			m.clearActiveSavedFilter()
			m.buildTree()
		},
		func(SortField) {
			m.clearActiveSavedFilter()
			m.sortNodes()
			m.updateVisibility()
		},
	)
	return handled, nil
}
