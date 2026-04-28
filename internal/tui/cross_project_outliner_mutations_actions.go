package tui

import (
	"fmt"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
)

// crossTogglePinned toggles the pinned state for a task in cross-project mode.
func (m *OutlinerModel) crossTogglePinned(row outlinerRow, proj *models.Project) (tea.Model, tea.Cmd) {
	m.crossUndoCtx.Set("Toggle Pin", row.task.Title)
	m.crossUndoCtx.ProjectIDs = []string{proj.ID}
	pf := m.crossPersistFn(proj)
	result := applyTogglePinned(pf, proj, row.task.ID)
	if result.Err != nil {
		return m, func() tea.Msg { return ErrorMsg{Err: result.Err} }
	}
	if result.StatusMessage != "" {
		m.setStatus(result.StatusMessage)
	}
	m.rebuildRows()
	return m, result.Cmd
}

// crossToggleDimmed toggles the dimmed state for a task in cross-project mode.
func (m *OutlinerModel) crossToggleDimmed(row outlinerRow, proj *models.Project) (tea.Model, tea.Cmd) {
	m.crossUndoCtx.Set("Toggle Dim", row.task.Title)
	m.crossUndoCtx.ProjectIDs = []string{proj.ID}
	pf := m.crossPersistFn(proj)
	result := applyToggleDimmed(pf, proj, row.task.ID)
	if result.Err != nil {
		return m, func() tea.Msg { return ErrorMsg{Err: result.Err} }
	}
	if result.StatusMessage != "" {
		m.setStatus(result.StatusMessage)
	}
	m.rebuildRows()
	return m, result.Cmd
}

// crossHandleSectionMove handles the m prefix in cross-project mode.
// Section keys (ma/mn/mi/mw/ms/mr/mS) move the cursor task to a section.
// mK/mJ delegate to moveTaskToPosition which already handles cross-project.
func (m *OutlinerModel) crossHandleSectionMove(key string, cursorTask *models.Task) (tea.Model, tea.Cmd) {
	if section, ok := sectionForMoveKey(key); ok {
		row, proj, valid := m.crossGetMutationTarget()
		if !valid {
			return m, nil
		}
		task := row.task
		label := "Move to " + section
		if section == "" {
			label = "Move to Active"
		}
		m.crossUndoCtx.Set(label, task.Title)
		m.crossUndoCtx.ProjectIDs = []string{proj.ID}
		svc := m.projectService
		if m.config != nil && m.config.UndoHistoryDepth != nil && *m.config.UndoHistoryDepth > 0 {
			_ = svc.RecordUndoSnapshot(proj, label, task.Title, *m.config.UndoHistoryDepth, nil)
		}
		if err := svc.Backup(proj); err != nil {
			return m, func() tea.Msg { return ErrorMsg{Err: fmt.Errorf("backup failed: %w", err)} }
		}
		proj.MutateTask(task.ID, func(t *models.Task) {
			t.SetSection(section)
			if section == "" {
				t.Activate()
			}
		})
		proj.RelocateTaskToEndOfSection(task.ID, section)
		if err := svc.Save(proj); err != nil {
			return m, func() tea.Msg { return ErrorMsg{Err: fmt.Errorf("save failed: %w", err)} }
		}
		_ = svc.Load(proj)
		m.rebuildRows()
		return m, nil
	}
	switch key {
	case "K":
		if cursorTask == nil {
			return m, nil
		}
		return m.moveTaskToPosition(cursorTask, true)
	case "J":
		if cursorTask == nil {
			return m, nil
		}
		return m.moveTaskToPosition(cursorTask, false)
	}
	return m, nil
}

// crossToggleActive toggles the active flag for a task in cross-project mode.
// Multi-select is not supported (tasks span projects); shows a status message instead.
func (m *OutlinerModel) crossToggleActive() (tea.Cmd, bool) {
	if m.multiSelectMode && len(m.selectedTasks) > 0 {
		m.setStatus("bulk toggle active not supported in cross-project mode")
		return nil, true
	}
	row, proj, valid := m.crossGetMutationTarget()
	if !valid {
		return nil, true
	}
	m.crossUndoCtx.Set("Toggle Active", row.task.Title)
	m.crossUndoCtx.ProjectIDs = []string{proj.ID}
	pf := m.crossPersistFn(proj)
	result := applyToggleActive(pf, proj, row.task.ID)
	if result.Err != nil {
		return func() tea.Msg { return ErrorMsg{Err: result.Err} }, true
	}
	if result.StatusMessage != "" {
		m.setStatus(result.StatusMessage)
	}
	m.rebuildRows()
	return result.Cmd, true
}

// crossHandleTypeChange handles <, >, H, L type change keys in cross-project mode.
// Multi-select is not supported (tasks span projects); shows a status message instead.
func (m *OutlinerModel) crossHandleTypeChange(key string) (tea.Cmd, bool) {
	op, undoAction, ok := typeChangeOpForKey(key)
	if !ok {
		return nil, false
	}
	if m.multiSelectMode && len(m.selectedTasks) > 0 {
		m.setStatus("bulk type change not supported in cross-project mode")
		return nil, true
	}
	row, proj, valid := m.crossGetMutationTarget()
	if !valid {
		return nil, true
	}
	m.crossUndoCtx.Set(undoAction, row.task.Title)
	m.crossUndoCtx.ProjectIDs = []string{proj.ID}
	pf := m.crossPersistFn(proj)
	result := applyTypeChange(pf, proj, row.task.ID, op)
	if result.Err != nil {
		return func() tea.Msg { return ErrorMsg{Err: result.Err} }, true
	}
	if result.StatusMessage != "" {
		m.setStatus(result.StatusMessage)
	}
	m.rebuildRows()
	return result.Cmd, true
}

// crossExecuteDelete performs a delete of m.deleteTarget in cross-project mode.
// It resolves the owning project via crossFindProjectForTask, calls Backup()
// and Save() on that project only, and forces a row rebuild via rebuildRows.
//
// Called from handleConfirmDialog when m.project==nil and the confirmed action
// is "delete" or "delete-recurring".
func (m *OutlinerModel) crossExecuteDelete() (tea.Model, tea.Cmd) {
	if m.deleteTarget == nil {
		return m, nil
	}
	task := m.deleteTarget
	m.deleteTarget = nil
	m.clearStatus()

	proj := m.crossFindProjectForTask(task.ID)
	if proj == nil {
		m.setStatus("Project for task not found")
		return m, nil
	}

	m.crossUndoCtx.Set("Delete Task", task.Title)
	m.crossUndoCtx.ProjectIDs = []string{proj.ID}
	deleteIDs := collectDescendantIDs(proj.Tasks, map[string]bool{task.ID: true})
	pf := m.crossPersistFn(proj)
	result := applyDeleteTask(pf, m.projectService, proj, task.ID)
	if result.Err != nil {
		m.err = result.Err
		return m, func() tea.Msg { return ErrorMsg{Err: result.Err} }
	}
	// Clean up config references (FocusQueue, DailyPlan, RecentTaskVisits)
	cleanupConfigTaskRefs(m.config, m.configManager, deleteIDs)
	m.rebuildRows()
	return m, result.Cmd
}