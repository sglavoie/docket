package tui

import (
	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
)

func (m *OutlinerModel) handleTaskMutationKey(key string, onHeader bool) (tea.Model, tea.Cmd, bool) {
	switch key {
	case "J":
		if len(m.rows) == 0 || m.cursor >= len(m.rows) || onHeader {
			return m, nil, true
		}
		task := m.rows[m.cursor].task
		if m.project == nil {
			row, proj, ok := m.crossGetMutationTarget()
			if !ok {
				return m, nil, true
			}
			task = row.task
			siblings := m.getSiblings(task, proj)
			idx := m.siblingIndex(siblings, task.ID)
			if idx >= 0 && idx < len(siblings)-1 {
				proj.SwapTasks(task, siblings[idx+1])
				m.crossUndoCtx.Set("Move Down", task.Title)
				m.crossUndoCtx.ProjectIDs = []string{proj.ID}
				m.rebuildRows()
				m.moveCursorToTask(task.ID)
				return m, m.crossSaveProject(proj), true
			}
			return m, nil, true
		}
		siblings := m.getSiblings(task, m.project)
		idx := m.siblingIndex(siblings, task.ID)
		if idx >= 0 && idx < len(siblings)-1 {
			taskID := task.ID
			m.project.SwapTasks(task, siblings[idx+1])
			m.setUndoContext("Move Down", task.Title)
			m.rebuildRows()
			m.moveCursorToTask(taskID)
			return m, m.saveWithWorkflow(), true
		}
		return m, nil, true

	case "K":
		if len(m.rows) == 0 || m.cursor >= len(m.rows) || onHeader {
			return m, nil, true
		}
		task := m.rows[m.cursor].task
		if m.project == nil {
			row, proj, ok := m.crossGetMutationTarget()
			if !ok {
				return m, nil, true
			}
			task = row.task
			siblings := m.getSiblings(task, proj)
			idx := m.siblingIndex(siblings, task.ID)
			if idx > 0 {
				proj.SwapTasks(task, siblings[idx-1])
				m.crossUndoCtx.Set("Move Up", task.Title)
				m.crossUndoCtx.ProjectIDs = []string{proj.ID}
				m.rebuildRows()
				m.moveCursorToTask(task.ID)
				return m, m.crossSaveProject(proj), true
			}
			return m, nil, true
		}
		siblings := m.getSiblings(task, m.project)
		idx := m.siblingIndex(siblings, task.ID)
		if idx > 0 {
			taskID := task.ID
			m.project.SwapTasks(task, siblings[idx-1])
			m.setUndoContext("Move Up", task.Title)
			m.rebuildRows()
			m.moveCursorToTask(taskID)
			return m, m.saveWithWorkflow(), true
		}
		return m, nil, true

	case "right":
		if len(m.rows) <= 1 || m.cursor <= 0 || m.cursor >= len(m.rows) || onHeader {
			return m, nil, true
		}
		if m.rows[m.cursor-1].task == nil {
			return m, nil, true
		}
		if m.project == nil {
			curRow := m.rows[m.cursor]
			aboveRow := m.rows[m.cursor-1]
			if curRow.ProjectID != aboveRow.ProjectID {
				return m, nil, true
			}
			proj := m.crossResolveProject(curRow.ProjectID)
			if proj == nil {
				return m, nil, true
			}
			task := proj.GetTaskByID(curRow.task.ID)
			candidate := proj.GetTaskByID(aboveRow.task.ID)
			if task == nil || candidate == nil {
				return m, nil, true
			}
			if IndentTask(proj, task, candidate) {
				m.crossUndoCtx.Set("Indent", task.Title)
				m.crossUndoCtx.ProjectIDs = []string{proj.ID}
				m.rebuildRows()
				return m, m.crossSaveProject(proj), true
			}
			return m, nil, true
		}
		task := m.project.GetTaskByID(m.rows[m.cursor].task.ID)
		candidate := m.project.GetTaskByID(m.rows[m.cursor-1].task.ID)
		if task == nil || candidate == nil {
			return m, nil, true
		}
		if IndentTask(m.project, task, candidate) {
			m.setUndoContext("Indent", task.Title)
			m.validateZoomStack()
			m.rebuildRows()
			return m, m.saveWithWorkflow(), true
		}
		return m, nil, true

	case "left":
		if len(m.rows) == 0 || m.cursor >= len(m.rows) || onHeader {
			return m, nil, true
		}
		if m.project == nil {
			curRow := m.rows[m.cursor]
			proj := m.crossResolveProject(curRow.ProjectID)
			if proj == nil {
				return m, nil, true
			}
			task := proj.GetTaskByID(curRow.task.ID)
			if task == nil {
				return m, nil, true
			}
			if OutdentTask(proj, task) {
				m.crossUndoCtx.Set("Outdent", task.Title)
				m.crossUndoCtx.ProjectIDs = []string{proj.ID}
				m.rebuildRows()
				return m, m.crossSaveProject(proj), true
			}
			return m, nil, true
		}
		task := m.project.GetTaskByID(m.rows[m.cursor].task.ID)
		if task == nil {
			return m, nil, true
		}
		if OutdentTask(m.project, task) {
			m.setUndoContext("Outdent", task.Title)
			m.validateZoomStack()
			m.rebuildRows()
			return m, m.saveWithWorkflow(), true
		}
		return m, nil, true
	}

	return nil, nil, false
}

func (m *OutlinerModel) siblingIndex(siblings []*models.Task, taskID string) int {
	for i, sibling := range siblings {
		if sibling.ID == taskID {
			return i
		}
	}
	return -1
}
