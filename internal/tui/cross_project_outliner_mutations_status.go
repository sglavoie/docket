package tui

import (
	"fmt"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
)

// crossHandleStatusMutation resolves the owning project from the cursor row,
// applies a status transition, and forces a row rebuild via rebuildRows.
// It is called from handlePrefixKey when prefix="," and m.project==nil.
//
// In multi-select mode with selections active, routes to crossApplyBulkStatusChange.
//
// Returns (model, cmd, true) when the status change was consumed.
// Returns (nil, nil, false) when not in cross-project mode.
func (m *OutlinerModel) crossHandleStatusMutation(status models.TaskStatus) (tea.Model, tea.Cmd, bool) {
	if m.project != nil {
		return nil, nil, false
	}
	if m.multiSelectMode && len(m.selectedTasks) > 0 {
		model, cmd := m.crossApplyBulkStatusChange(status)
		return model, cmd, true
	}
	row, proj, valid := m.crossGetMutationTarget()
	if !valid {
		return m, nil, true
	}
	m.crossUndoCtx.Set(StatusVerb(status)+" Task", row.task.Title)
	m.crossUndoCtx.ProjectIDs = []string{proj.ID}
	pf := m.crossPersistFn(proj)
	result := applyStatusChange(pf, proj, row.task.ID, status)
	if result.Err != nil {
		return m, func() tea.Msg { return ErrorMsg{Err: result.Err} }, true
	}
	if result.StatusMessage != "" {
		m.setStatus(result.StatusMessage)
	}
	if status == models.StatusDone {
		m.setCompletionFlash()
		return m, tea.Batch(result.Cmd, completionFlashScheduler()), true
	}
	m.setMutationFlash(statusMutationFlashBg(status))
	return m, tea.Batch(result.Cmd, mutationFlashTimeout()), true
}

// crossApplyBulkStatusChange collects selected tasks from current rows, shows a
// confirmation dialog with the aggregate count, and stores the pending operation
// in m.bulkOp for execution after the user confirms.
func (m *OutlinerModel) crossApplyBulkStatusChange(newStatus models.TaskStatus) (tea.Model, tea.Cmd) {
	var tasks []*models.Task
	for _, row := range m.rows {
		if row.IsSeparator || row.IsSectionHeader || row.task == nil {
			continue
		}
		if m.selectedTasks[row.task.ID] {
			tasks = append(tasks, row.task)
		}
	}
	if len(tasks) == 0 {
		return m, nil
	}
	m.bulkOp.SetOperation(newStatus, tasks, 0, nil)
	m.confirm.Show("bulk-status", fmt.Sprintf("Change %d tasks to %s?", len(tasks), string(newStatus)))
	m.clearMultiSelect()
	return m, nil
}

// crossExecutePendingBulkStatus applies the pending cross-project bulk status
// change after the user confirms the dialog. It groups the pending tasks by
// project, calls Backup() then Save() per project, and halts on first failure
// without processing remaining projects. All affected project IDs and backup
// paths are stored in m.crossUndoCtx for undo.
func (m *OutlinerModel) crossExecutePendingBulkStatus() (tea.Model, tea.Cmd) {
	if len(m.bulkOp.GetTasks()) == 0 {
		m.bulkOp.Clear()
		return m, nil
	}

	status := m.bulkOp.GetStatus()
	tasks := m.bulkOp.GetTasks()

	// Group tasks by project.
	type projectGroup struct {
		proj  *models.Project
		tasks []*models.Task
	}
	grouped := make(map[string]*projectGroup)
	var projectOrder []string
	for _, task := range tasks {
		proj := m.crossFindProjectForTask(task.ID)
		if proj == nil {
			continue
		}
		if _, exists := grouped[proj.ID]; !exists {
			grouped[proj.ID] = &projectGroup{proj: proj}
			projectOrder = append(projectOrder, proj.ID)
		}
		grouped[proj.ID].tasks = append(grouped[proj.ID].tasks, task)
	}

	svc := m.projectService
	var projectIDs []string

	undoDepth := 0
	if m.config != nil && m.config.UndoHistoryDepth != nil {
		undoDepth = *m.config.UndoHistoryDepth
	}
	projectsToSnapshot := make([]*models.Project, 0, len(projectOrder))
	for _, projID := range projectOrder {
		projectsToSnapshot = append(projectsToSnapshot, grouped[projID].proj)
	}
	groupID, groupProjectIDs := undoGroupMetadataForProjects(projectsToSnapshot)

	for _, projID := range projectOrder {
		group := grouped[projID]
		if undoDepth > 0 {
			if err := recordTUIUndoSnapshotWithGroup(svc, group.proj, "Bulk Status Change",
				fmt.Sprintf("%d tasks -> %s", len(group.tasks), status), undoDepth, movedTaskIDs(group.tasks), groupID, groupProjectIDs); err != nil {
				m.bulkOp.Clear()
				return m, func() tea.Msg { return ErrorMsg{Err: err} }
			}
		}
		if err := svc.Backup(group.proj); err != nil {
			m.bulkOp.Clear()
			return m, func() tea.Msg { return ErrorMsg{Err: fmt.Errorf("backup failed: %w", err)} }
		}
		projectIDs = append(projectIDs, projID)
		for _, task := range group.tasks {
			t := group.proj.GetTaskByID(task.ID)
			if t == nil || t.CanTransitionTo(status) != "" {
				continue
			}
			group.proj.MutateTask(task.ID, func(t *models.Task) {
				t.ApplyTransition(status)
			})
		}
		if err := svc.Save(group.proj); err != nil {
			m.bulkOp.Clear()
			return m, func() tea.Msg { return ErrorMsg{Err: fmt.Errorf("save failed: %w", err)} }
		}
		_ = svc.Load(group.proj)
	}

	m.crossUndoCtx.Set("Bulk Status Change", fmt.Sprintf("%d tasks -> %s", len(tasks), status))
	m.crossUndoCtx.ProjectIDs = projectIDs
	m.crossUndoCtx.GroupID = groupID

	m.setStatus(fmt.Sprintf("Changed %d tasks to %s", len(tasks), status))
	m.bulkOp.Clear()
	m.rebuildRows()
	return m, nil
}
