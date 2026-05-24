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

// crossSendTaskToSection moves the cursor task to the specified section in cross-project mode.
func (m *OutlinerModel) crossSendTaskToSection(section string) (tea.Model, tea.Cmd) {
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
	m.mutationFlash = true
	if updated := proj.GetTaskByID(row.task.ID); updated != nil {
		m.mutationFlashBg = typeMutationFlashBg(updated.Type)
	}
	m.rebuildRows()
	return tea.Batch(result.Cmd, mutationFlashTimeout()), true
}

// handleCrossProjectReparent applies a ParentSelectedMsg in cross-project mode.
// Resolves the owning project via crossFindProjectForTask, records an undo
// snapshot, mutates the parent pointer with cycle / type-compat guards, and
// persists via crossSaveProject.
func (m *OutlinerModel) handleCrossProjectReparent(msg ParentSelectedMsg) (tea.Model, tea.Cmd) {
	proj := m.crossFindProjectForTask(msg.ReparentTaskID)
	if proj == nil {
		return m, nil
	}
	task := proj.GetTaskByID(msg.ReparentTaskID)
	if task == nil {
		return m, nil
	}

	m.crossUndoCtx.Set("Reparent Task", task.Title)
	m.crossUndoCtx.ProjectIDs = []string{proj.ID}
	if m.config != nil && m.config.UndoHistoryDepth != nil && *m.config.UndoHistoryDepth > 0 {
		_ = m.projectService.RecordUndoSnapshot(proj, "Reparent Task", task.Title, *m.config.UndoHistoryDepth, nil)
	}

	if msg.ParentID == nil {
		proj.MutateTask(task.ID, func(t *models.Task) {
			t.ParentID = nil
		})
	} else {
		targetID := *msg.ParentID
		taskMap := make(map[string]*models.Task, len(proj.Tasks))
		for _, t := range proj.Tasks {
			taskMap[t.ID] = t
		}
		targetParent := taskMap[targetID]
		if targetParent != nil && task.Type.CanBeChildOf(targetParent.Type) && !models.WouldCreateCycle(taskMap, task.ID, targetID) {
			proj.MutateTask(task.ID, func(t *models.Task) {
				t.ParentID = &targetID
			})
		}
	}
	proj.BuildTaskTree()
	return m, m.crossSaveProject(proj)
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
