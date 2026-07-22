package tui

import (
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

// cursorProject returns the project that owns the row under the cursor.
// Single-project mode returns m.project; cross-project mode resolves
// row.ProjectID via m.loadedProjects. Returns nil for empty rows, an
// out-of-range cursor, section headers, separators, project sub-headers,
// rows without a task, or stale project IDs that no longer match any loaded
// project.
func (m *OutlinerModel) cursorProject() *models.Project {
	if len(m.rows) == 0 || m.cursor < 0 || m.cursor >= len(m.rows) {
		return nil
	}
	row := m.rows[m.cursor]
	if !row.isTaskRow() || row.task == nil {
		return nil
	}
	if m.project != nil {
		return m.project
	}
	return m.crossResolveProject(row.ProjectID)
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
}

// toggleMultiSelectMode enters or exits multi-select mode.
func (m *OutlinerModel) toggleMultiSelectMode() {
	m.multiSelectMode = !m.multiSelectMode
	if !m.multiSelectMode {
		m.ClearSelection()
		m.clearStatus()
		return
	}

	m.setStatus("Multi-select: Space=toggle, a=all, A=deselect, X=delete, v/Esc=exit")
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

// deselectAllVisible removes all currently visible rows from the selection.
func (m *OutlinerModel) deselectAllVisible() {
	for _, row := range m.rows {
		if !row.isTaskRow() || row.task == nil {
			continue
		}
		m.DeselectTaskIdentity(m.taskIdentityForRow(row))
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

// getSelectedTasks returns a slice of selected tasks.
// In cross-project mode (m.project == nil), tasks are collected from m.rows
// to avoid a nil dereference on m.project.
func (m *OutlinerModel) getSelectedTasks() []*models.Task {
	if m.project == nil {
		var tasks []*models.Task
		for _, row := range m.rows {
			if !row.isTaskRow() || row.task == nil {
				continue
			}
			if m.isRowSelected(row) {
				tasks = append(tasks, row.task)
			}
		}
		return tasks
	}

	var tasks []*models.Task
	for _, task := range m.project.Tasks {
		row := outlinerRow{task: task}
		if m.isRowSelected(row) {
			tasks = append(tasks, task)
		}
	}
	return tasks
}

// getSelectedTaskProjectRefs returns selected task refs with their owning
// projects. Same-project views use the loaded project; cross-project views
// resolve each visible selected row to its row project.
func (m *OutlinerModel) getSelectedTaskProjectRefs() []TaskProjectRef {
	if !m.multiSelectMode || m.SelectedCount() == 0 {
		return nil
	}
	if m.project != nil {
		return taskProjectRefsFromTasks(m.getSelectedTasks(), m.project)
	}

	refs := make([]TaskProjectRef, 0, m.SelectedCount())
	for _, row := range m.rows {
		if !row.isTaskRow() || row.task == nil {
			continue
		}
		if !m.isRowSelected(row) {
			continue
		}
		project := m.crossResolveProject(row.ProjectID)
		if project == nil {
			continue
		}
		refs = append(refs, TaskProjectRef{Task: row.task, Project: project})
	}
	return refs
}

// clearMultiSelect exits multi-select mode and clears all selections.
func (m *OutlinerModel) clearMultiSelect() {
	m.ExitMultiSelect()
	m.clearStatus()
}

// getMultiSelectedTasks returns selected tasks when in multi-select mode, nil otherwise.
func (m *OutlinerModel) getMultiSelectedTasks() []*models.Task {
	if m.multiSelectMode && m.SelectedCount() > 0 {
		return m.getSelectedTasks()
	}
	return nil
}
