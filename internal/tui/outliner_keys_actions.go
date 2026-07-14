package tui

import (
	"fmt"
	"maps"
	"slices"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/logging"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

// handleConfirmDialog processes key input while a confirmation dialog is active.
func (m *OutlinerModel) handleConfirmDialog(key string) (tea.Model, tea.Cmd) {
	// Capture action before HandleKey clears it on confirm/cancel.
	action := m.confirm.Action()

	// Handle recurring delete separately (d/s/n instead of y/n)
	if action == "delete-recurring" && m.deleteTarget != nil {
		switch key {
		case "d", "D":
			m.confirm.Cancel()
			m.clearStatus()
			if m.project == nil {
				return m.crossExecuteDelete()
			}
			return m.executeDelete()
		case "s", "S":
			// Stop recurrence but keep the task (don't delete). Route through the
			// app-layer ClearRecurrence seam (one ledger transaction); the shell
			// reloads and toasts. Same path in single-project and cross-project mode.
			task := m.deleteTarget
			m.deleteTarget = nil
			m.confirm.Cancel()
			m.clearStatus()
			return m, m.taskItem.ClearRecurrence(task.ID)
		case "n", "N", "esc":
			m.confirm.Cancel()
			m.clearStatus()
			m.deleteTarget = nil
			return m, nil
		}
		return m, nil
	}

	// Standard y/n handling
	if confirmed, handled := m.HandleConfirmKey(key); handled {
		if confirmed {
			switch action {
			case projectArchiveConfirmAction:
				return m, func() tea.Msg { return projectArchiveConfirmedMsg{} }
			case "delete":
				if m.project == nil {
					return m.crossExecuteDelete()
				}
				return m.executeDelete()
			case "bulk-delete":
				return m.executeBulkDelete()
			case "bulk-status":
				if m.project == nil {
					return m.crossExecutePendingBulkStatus()
				}
				return m.executePendingBulkStatus()
			case "archive-tasks":
				return m.archiveCompletedTasks()
			}
		}
		m.deleteTarget = nil
		m.archiveTargetProject = nil
		m.bulkOp.Clear()
		return m, nil
	}
	return m, nil
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

// showDeleteConfirm shows a confirmation dialog for deleting a single task.
func (m *OutlinerModel) showDeleteConfirm(task *models.Task) {
	m.deleteTarget = task
	if task.IsRecurring() {
		m.confirm.Show("delete-recurring",
			fmt.Sprintf("Delete recurring task #%d '%s'? (d)elete instance / (s)top recurrence / (n)o",
				task.ShortID, task.Title))
		return
	}

	childCount := task.CountChildren()
	detail := &ConfirmDetail{
		Items:    []string{fmt.Sprintf("#%d %s (%s)", task.ShortID, task.Title, task.Type)},
		UndoHint: true,
	}
	if childCount > 0 {
		label := "subtask"
		if childCount != 1 {
			label = "subtasks"
		}
		detail.CascadeNote = fmt.Sprintf("%d %s will also be deleted", childCount, label)
	}
	m.confirm.ShowWithDetail("delete", "Delete this task?", detail)
}

// showBulkDeleteConfirm shows a confirmation dialog for deleting multiple selected tasks.
func (m *OutlinerModel) showBulkDeleteConfirm() {
	selected := m.getSelectedTasks()
	totalChildren := 0
	for _, task := range selected {
		totalChildren += task.CountChildren()
	}

	const maxItems = 5
	detail := &ConfirmDetail{
		TotalCount: len(selected),
		UndoHint:   true,
	}
	for i, task := range selected {
		if i >= maxItems {
			break
		}
		detail.Items = append(detail.Items, fmt.Sprintf("#%d %s", task.ShortID, task.Title))
	}
	if totalChildren > 0 {
		label := "subtask"
		if totalChildren != 1 {
			label = "subtasks"
		}
		detail.CascadeNote = fmt.Sprintf("%d %s will also be deleted", totalChildren, label)
	}

	m.confirm.ShowWithDetail("bulk-delete",
		fmt.Sprintf("Delete %d selected tasks?", len(selected)), detail)
}

// executeDelete removes the current deleteTarget task through the
// application-layer BulkCoordinator (see the body comment for the transaction
// and config-artifact semantics).
func (m *OutlinerModel) executeDelete() (tea.Model, tea.Cmd) {
	if m.deleteTarget == nil {
		return m, nil
	}
	task := m.deleteTarget
	m.deleteTarget = nil
	m.clearStatus()

	// Route the single-task delete through the application-layer BulkCoordinator
	// (single-element), mirroring crossExecuteDelete. The durable planning-ref
	// cleanup (Focus Queue, Daily Plan, dismissals) of the removed subtree rides
	// the delete transaction as a config artifact so it is atomic and undoable
	// through the ledger; machine-local recent visits are repaired by the shell
	// after the commit from the batch's RemovedTaskIDs.
	deleteIDs := collectDescendantIDs(m.project.Tasks, map[string]bool{task.ID: true})
	var artifacts []storage.ArtifactChange
	if m.runtime != nil && m.configManager != nil {
		planning := clonePlanningState(m.runtime.Planning())
		if planning.RemoveDurableTaskRefs(deleteIDs) > 0 {
			arts, err := planningDomainArtifact(m.configManager, planning)
			if err != nil {
				m.setStatus(fmt.Sprintf("Error: %v", err))
				return m, nil
			}
			artifacts = arts
		}
	}
	return m, m.bulk.Delete([]string{task.ID}, artifacts...)
}

// executeBulkDelete removes all multi-selected tasks using shared helpers.
// Collects all tasks and descendants, cleans up attachments, filters in one
// pass, then persists once instead of per-task.
func (m *OutlinerModel) executeBulkDelete() (tea.Model, tea.Cmd) {
	if len(m.selectedTasks) == 0 {
		return m, nil
	}

	count := len(m.selectedTasks)
	prevStates := captureTaskStates(m.project.Tasks)

	// Collect all selected task IDs and their descendants
	rootIDs := make(map[string]bool, len(m.selectedTasks))
	for taskID := range m.selectedTasks {
		rootIDs[taskID] = true
	}
	idsToDelete := collectDescendantIDs(m.project.Tasks, rootIDs)

	// Clean up attachments for all tasks being deleted
	for _, t := range m.project.Tasks {
		if idsToDelete[t.ID] {
			if err := storage.CleanupTaskAttachments(m.project, t); err != nil {
				logging.Warn("failed to clean up attachments for task %s: %v", t.ID, err)
			}
		}
	}

	// Filter out deleted tasks and clear dangling references
	var remaining []*models.Task
	for _, t := range m.project.Tasks {
		if !idsToDelete[t.ID] {
			if t.ParentID != nil && idsToDelete[*t.ParentID] {
				t.ParentID = nil
			}
			// Clear dangling DependsOn references to deleted tasks
			if len(t.DependsOn) > 0 {
				cleaned := t.DependsOn[:0]
				for _, depID := range t.DependsOn {
					if !idsToDelete[depID] {
						cleaned = append(cleaned, depID)
					}
				}
				t.DependsOn = cleaned
			}
			// Clear dangling RelatedTo references to deleted tasks
			if len(t.RelatedTo) > 0 {
				cleaned := t.RelatedTo[:0]
				for _, relID := range t.RelatedTo {
					if !idsToDelete[relID] {
						cleaned = append(cleaned, relID)
					}
				}
				t.RelatedTo = cleaned
			}
			remaining = append(remaining, t)
		}
	}
	m.project.Tasks = remaining

	// Commit the filtered project once through the application layer.
	m.setUndoContext("Bulk Delete", fmt.Sprintf("%d tasks", count))
	m.setUndoTaskIDs(slices.Collect(maps.Keys(idsToDelete)))
	cmd := m.commitWorkflow(prevStates, fmt.Sprintf("Deleted %d tasks", count))

	// Clean up config references (FocusQueue, DailyPlan, RecentTaskVisits)
	if err := cleanupRuntimeTaskRefs(m.runtime, idsToDelete); err != nil {
		m.setStatus(fmt.Sprintf("Error: %v", err))
	}

	m.clearMultiSelect()
	return m, cmd
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
