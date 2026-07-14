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

// toggleMultiSelectMode enters or exits multi-select mode.
func (m *OutlinerModel) toggleMultiSelectMode() {
	m.multiSelectMode = !m.multiSelectMode
	if !m.multiSelectMode {
		m.ClearSelection()
		m.clearStatus()
		return
	}

	m.setStatus("Multi-select: Space=toggle, a=all, A=deselect, D=delete, v/Esc=exit")
}

// selectAllVisible adds all currently visible rows to the selection.
func (m *OutlinerModel) selectAllVisible() {
	for _, row := range m.rows {
		if !row.isTaskRow() || row.task == nil {
			continue
		}
		m.selectedTasks[row.task.ID] = true
	}
}

// deselectAllVisible removes all currently visible rows from the selection.
func (m *OutlinerModel) deselectAllVisible() {
	for _, row := range m.rows {
		if !row.isTaskRow() || row.task == nil {
			continue
		}
		delete(m.selectedTasks, row.task.ID)
	}
}

// pruneInvisibleSelections removes selected tasks that are no longer visible.
func (m *OutlinerModel) pruneInvisibleSelections() {
	if !m.multiSelectMode {
		return
	}
	visibleIDs := make(map[string]bool, len(m.rows))
	for _, row := range m.rows {
		if !row.isTaskRow() || row.task == nil {
			continue
		}
		visibleIDs[row.task.ID] = true
	}
	for id := range m.selectedTasks {
		if !visibleIDs[id] {
			delete(m.selectedTasks, id)
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
			if m.selectedTasks[row.task.ID] {
				tasks = append(tasks, row.task)
			}
		}
		return tasks
	}

	var tasks []*models.Task
	for _, task := range m.project.Tasks {
		if m.selectedTasks[task.ID] {
			tasks = append(tasks, task)
		}
	}
	return tasks
}

// getSelectedTaskProjectRefs returns selected task refs with their owning
// projects. Same-project views use the loaded project; cross-project views
// resolve each visible selected row to its row project.
func (m *OutlinerModel) getSelectedTaskProjectRefs() []TaskProjectRef {
	if !m.multiSelectMode || len(m.selectedTasks) == 0 {
		return nil
	}
	if m.project != nil {
		return taskProjectRefsFromTasks(m.getSelectedTasks(), m.project)
	}

	refs := make([]TaskProjectRef, 0, len(m.selectedTasks))
	for _, row := range m.rows {
		if !row.isTaskRow() || row.task == nil {
			continue
		}
		if !m.selectedTasks[row.task.ID] {
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
	if m.multiSelectMode && len(m.selectedTasks) > 0 {
		return m.getSelectedTasks()
	}
	return nil
}
