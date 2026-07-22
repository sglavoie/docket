package tui

import (
	"fmt"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
)

// handleConfirmDialog processes key input while a confirmation dialog is active.
//
// Task deletion is no longer confirmed here: the Outliner emits the canonical,
// shell-owned TaskDeleteRequestedMsg (see taskDeleteRequest), so this handler
// only owns the Outliner-local confirmations that have no shell equivalent
// (project archive, bulk status change, archive-completed).
func (m *OutlinerModel) handleConfirmDialog(key string) (tea.Model, tea.Cmd) {
	// Capture action before HandleKey clears it on confirm/cancel.
	action := m.confirm.Action()

	// Standard y/n handling
	if confirmed, handled := m.HandleConfirmKey(key); handled {
		if confirmed {
			switch action {
			case projectArchiveConfirmAction:
				return m, func() tea.Msg { return projectArchiveConfirmedMsg{} }
			case "bulk-status":
				if m.project == nil {
					return m.crossExecutePendingBulkStatus()
				}
				return m.executePendingBulkStatus()
			case "archive-tasks":
				return m.archiveCompletedTasks()
			}
		}
		m.archiveTargetProject = nil
		m.bulkOp.Clear()
		return m, nil
	}
	return m, nil
}

// taskDeleteRequest builds the canonical, shell-owned deletion request for the
// Outliner's current target: every multi-selected task when a selection is
// active, otherwise the single cursor task. It resolves each task's owning
// project (single-project uses m.project; cross-project resolves per row) and
// stamps the originating presentation so the shell repairs the right cursor
// after commit. It returns nil when there is no deletable target, and never
// mutates task data — the shell confirms and runs the atomic transaction.
func (m *OutlinerModel) taskDeleteRequest() tea.Msg {
	origin := ViewOutliner
	if m.project == nil {
		origin = ViewCrossProjectOutliner
	}
	if refs := m.getSelectedTaskProjectRefs(); len(refs) > 0 {
		return TaskDeleteRequestedMsg{Refs: refs, Origin: origin}
	}
	task, project := m.getTaskRef()
	if task == nil || project == nil {
		return nil
	}
	return TaskDeleteRequestedMsg{Refs: []TaskProjectRef{{Task: task, Project: project}}, Origin: origin}
}

// requestTaskDelete wraps taskDeleteRequest as a keyboard action, returning a
// command that emits the canonical request (or a no-op when there is no target).
func (m *OutlinerModel) requestTaskDelete() (tea.Model, tea.Cmd) {
	msg := m.taskDeleteRequest()
	if msg == nil {
		return m, nil
	}
	return m, func() tea.Msg { return msg }
}

// applyBulkStateChange applies a workflow-state transition to all multi-selected
// tasks. If some tasks are blocked by transition rules or incomplete
// dependencies, shows a confirmation prompt before proceeding with the valid ones.
func (m *OutlinerModel) applyBulkStateChange(newState models.WorkflowState) (tea.Model, tea.Cmd) {
	tasks := m.getSelectedTasks()
	if len(tasks) == 0 {
		return m, nil
	}

	valid, blockedCount := analyzeBulkStateTransition(m.project, tasks, newState)
	if len(valid) == 0 {
		m.setStatus("No selected tasks can transition to " + string(newState))
		m.clearMultiSelect()
		return m, nil
	}

	prevStates := captureTaskStates(m.project.Tasks)

	// If some tasks are blocked, show confirmation before proceeding
	if blockedCount > 0 {
		m.bulkOp.SetOperation(newState, valid, blockedCount, prevStates)
		m.confirm.Show("bulk-status",
			fmt.Sprintf("%d tasks blocked by dependencies. Apply to %d valid tasks?", blockedCount, len(valid)))
		m.clearMultiSelect()
		return m, nil
	}

	// No blocked tasks: mutate all in memory, then commit once through the app layer.
	for _, task := range valid {
		m.project.MutateTask(task.ID, func(t *models.Task) {
			t.ApplyStateTransition(newState)
		})
	}
	m.setUndoContext("Bulk Status Change", fmt.Sprintf("%d tasks -> %s", len(valid), newState))
	m.undo.TaskIDs = taskIDsFromSlice(valid)
	m.clearMultiSelect()
	return m, m.commitWorkflow(prevStates, fmt.Sprintf("Changed %d tasks to %s", len(valid), newState))
}

// executePendingBulkStatus applies the pending bulk state change after confirmation.
func (m *OutlinerModel) executePendingBulkStatus() (tea.Model, tea.Cmd) {
	if len(m.bulkOp.GetTasks()) == 0 {
		m.bulkOp.Clear()
		return m, nil
	}

	prevStates := m.bulkOp.GetPrevStates()
	state := m.bulkOp.GetState()
	tasks := m.bulkOp.GetTasks()

	for _, task := range tasks {
		m.project.MutateTask(task.ID, func(t *models.Task) {
			t.ApplyStateTransition(state)
		})
	}
	m.setUndoContext("Bulk Status Change", fmt.Sprintf("%d tasks -> %s", len(tasks), state))
	m.undo.TaskIDs = taskIDsFromSlice(tasks)
	m.bulkOp.Clear()
	return m, m.commitWorkflow(prevStates, fmt.Sprintf("Changed %d tasks to %s", len(tasks), state))
}

// archiveCompletedTasks moves all completed tasks to the archive file.
// The target project is m.archiveTargetProject when set (used by cross-project
// mode to capture the cursor's owning project at confirmation time); otherwise
// it falls back to m.project for single-project mode.
func (m *OutlinerModel) archiveCompletedTasks() (tea.Model, tea.Cmd) {
	proj := m.archiveTargetProject
	m.archiveTargetProject = nil
	if proj == nil {
		proj = m.project
	}
	if proj == nil {
		return m, nil
	}

	undoDepth := 0
	if m.runtime.Settings().UndoHistoryDepth != nil {
		undoDepth = *m.runtime.Settings().UndoHistoryDepth
	}
	completedCount := proj.CountCompletedTasks()

	if completedCount == 0 {
		return m, nil
	}

	svc := m.projectService
	cmd := func() tea.Msg {
		if undoDepth > 0 {
			_ = svc.RecordUndoSnapshot(proj, "Archive Tasks",
				fmt.Sprintf("%d tasks", completedCount), undoDepth, nil)
		}
		if err := svc.Backup(proj); err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to create backup: %w", err)}
		}
		count, err := svc.ArchiveTasks(proj)
		if err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to archive tasks: %w", err)}
		}
		_ = svc.Load(proj)
		// Note: rebuildRows() is called by TasksArchivedMsg handler
		return TasksArchivedMsg{Count: count}
	}
	m.clearStatus()
	return m, cmd
}
