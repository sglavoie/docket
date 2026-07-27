package tui

import (
	"fmt"

	"github.com/sglavoie/tt/internal/models"
)

// taskOpsView implementation for OutlinerModel.

func (m *OutlinerModel) getTaskRef() (*models.Task, *models.Project) {
	if len(m.rows) == 0 || m.cursor < 0 || m.cursor >= len(m.rows) {
		return nil, nil
	}

	row := m.rows[m.cursor]
	if !row.isTaskRow() {
		return nil, nil
	}
	if m.project != nil {
		return row.task, m.project
	}

	for _, proj := range m.loadedProjects {
		if proj.ID == row.ProjectID {
			return row.task, proj
		}
	}
	return nil, nil
}

func (m *OutlinerModel) setStatus(msg string) { m.InteractiveViewState.setStatus(msg) }

func (m *OutlinerModel) taskIdentityForRow(row outlinerRow) TaskIdentity {
	if row.task == nil {
		return TaskIdentity{}
	}
	project := m.project
	if project == nil {
		project = m.crossResolveProject(row.ProjectID)
	}
	return taskIdentityFor(row.task, project)
}

func (m *OutlinerModel) isRowSelected(row outlinerRow) bool {
	return m.IsTaskIdentitySelected(m.taskIdentityForRow(row))
}

func (m *OutlinerModel) toggleCursorSelection() {
	if m.cursor < 0 || m.cursor >= len(m.rows) || !m.rows[m.cursor].isTaskRow() {
		return
	}
	m.ToggleTaskIdentity(m.taskIdentityForRow(m.rows[m.cursor]))
	m.updateVisualSelectionStatus()
}

// toggleMultiSelectMode enters or exits multi-select mode.
func (m *OutlinerModel) toggleMultiSelectMode() {
	m.multiSelectMode = !m.multiSelectMode
	if !m.multiSelectMode {
		m.ClearSelection()
		m.clearStatus()
		return
	}

	m.updateVisualSelectionStatus()
}

func (m *OutlinerModel) updateVisualSelectionStatus() {
	if !m.multiSelectMode {
		return
	}
	if count := m.SelectedCount(); count > 0 {
		m.setStatus(fmt.Sprintf("Visual selection: %d selected · Space toggle · Enter actions · Esc/v exit", count))
		return
	}
	m.setStatus("Visual selection: Space toggle · Enter actions · Esc/v exit")
}

// selectAllVisible adds all currently visible rows to the selection.
func (m *OutlinerModel) selectAllVisible() {
	for _, row := range m.rows {
		if !row.isTaskRow() || row.task == nil {
			continue
		}
		m.SelectTaskIdentity(m.taskIdentityForRow(row))
	}
}

// pruneInvisibleSelections removes selected tasks that are no longer visible.
func (m *OutlinerModel) pruneInvisibleSelections() {
	if !m.multiSelectMode {
		return
	}
	visibleIdentities := make(map[TaskIdentity]bool, len(m.rows))
	for _, row := range m.rows {
		if !row.isTaskRow() || row.task == nil {
			continue
		}
		visibleIdentities[m.taskIdentityForRow(row)] = true
	}
	for identity := range m.selectedTaskIdentities {
		if !visibleIdentities[identity] {
			delete(m.selectedTaskIdentities, identity)
		}
	}
}

// clearMultiSelect exits multi-select mode and clears all selections.
func (m *OutlinerModel) clearMultiSelect() {
	m.ExitMultiSelect()
	m.clearStatus()
}
