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
	if row.IsSectionHeader || row.IsSeparator {
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

func (m *OutlinerModel) getPersister() Persister { return m.projectService }
func (m *OutlinerModel) reloadData()             { m.rebuildRows() }
func (m *OutlinerModel) setErr(err error)        { m.err = err }
func (m *OutlinerModel) setStatus(msg string)    { m.InteractiveViewState.setStatus(msg) }

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
		if row.IsSectionHeader || row.IsSeparator || row.task == nil {
			continue
		}
		m.selectedTasks[row.task.ID] = true
	}
}

// deselectAllVisible removes all currently visible rows from the selection.
func (m *OutlinerModel) deselectAllVisible() {
	for _, row := range m.rows {
		if row.IsSectionHeader || row.IsSeparator || row.task == nil {
			continue
		}
		delete(m.selectedTasks, row.task.ID)
	}
}

// getSelectedTasks returns a slice of selected tasks.
// In cross-project mode (m.project == nil), tasks are collected from m.rows
// to avoid a nil dereference on m.project.
func (m *OutlinerModel) getSelectedTasks() []*models.Task {
	if m.project == nil {
		var tasks []*models.Task
		for _, row := range m.rows {
			if row.IsSectionHeader || row.IsSeparator || row.task == nil {
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
