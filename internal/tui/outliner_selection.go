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
	m.peekMode = false
	m.multiSelectMode = true
	m.ToggleTaskIdentity(m.taskIdentityForRow(m.rows[m.cursor]))
	m.updateVisualSelectionStatus()
}

func (m *OutlinerModel) togglePeek() bool {
	if m == nil || m.multiSelectMode {
		return false
	}
	if m.peekMode {
		m.peekMode = false
		m.resetViewScrollOffset()
		return true
	}
	if task, _ := m.getTaskRef(); task == nil {
		return false
	}
	m.peekMode = true
	m.resetViewScrollOffset()
	return true
}

// toggleMultiSelectMode enters or exits multi-select mode.
func (m *OutlinerModel) toggleMultiSelectMode() {
	m.multiSelectMode = !m.multiSelectMode
	if !m.multiSelectMode {
		m.ClearSelection()
		m.clearStatus()
		return
	}

	m.peekMode = false
	m.updateVisualSelectionStatus()
}

func (m *OutlinerModel) updateVisualSelectionStatus() {
	if !m.multiSelectMode {
		return
	}
	if count := m.SelectedCount(); count > 0 {
		m.setStatus(fmt.Sprintf("Visual selection: %d selected · v toggle · Space delete menu · Esc clear", count))
		return
	}
	m.setStatus("Visual selection: v toggle · V all visible · Esc clear")
}

// selectAllVisible adds all currently visible rows to the selection.
func (m *OutlinerModel) selectAllVisible() {
	m.peekMode = false
	for _, row := range m.rows {
		if !row.isTaskRow() || row.task == nil {
			continue
		}
		m.SelectTaskIdentity(m.taskIdentityForRow(row))
	}
}

// pruneInvisibleSelections intentionally retains valid project-qualified
// selections even when a filter, mode, or successful mutation hides their rows.
// Non-delete bulk actions preserve the frozen selection so a follow-up action
// never narrows silently to what happens to remain visible.
func (m *OutlinerModel) pruneInvisibleSelections() {
}

// clearMultiSelect exits multi-select mode and clears all selections.
func (m *OutlinerModel) clearMultiSelect() {
	m.ExitMultiSelect()
	m.clearStatus()
}
