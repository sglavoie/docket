package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/workflow"
)

func (m *ProjectTreeModel) archiveProject() tea.Cmd {
	return func() tea.Msg {
		m.project.Archive()

		// Clear current project if it was the archived one
		if m.config.CurrentProject == m.project.ID {
			m.config.CurrentProject = ""
		}

		if err := m.configManager.Save(m.config); err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to save: %w", err)}
		}

		return ProjectArchivedMsg{Project: m.project}
	}
}

func (m *ProjectTreeModel) unarchiveProject() tea.Cmd {
	return func() tea.Msg {
		m.project.Unarchive()

		if err := m.configManager.Save(m.config); err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to save: %w", err)}
		}

		return ProjectArchivedMsg{Project: m.project}
	}
}

// performUndo restores the project from backup.
func (m *ProjectTreeModel) performUndo() tea.Cmd {
	return func() tea.Msg {
		if !storage.BackupExists(m.project) {
			return ErrorMsg{Err: fmt.Errorf("no backup available to undo")}
		}

		tasksBefore := len(m.project.Tasks)

		if err := storage.RestoreFromBackup(m.project); err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to undo: %w", err)}
		}

		if err := storage.LoadProject(m.project); err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to reload after undo: %w", err)}
		}

		m.buildTree()
		return UndoCompletedMsg{
			TasksBefore: tasksBefore,
			TasksAfter:  len(m.project.Tasks),
		}
	}
}

// setUndoContext sets the action context for the next save operation.
func (m *ProjectTreeModel) setUndoContext(action, context string) {
	m.pendingUndoAction = action
	m.pendingUndoContext = context
}

// clearUndoContext clears the pending undo context.
func (m *ProjectTreeModel) clearUndoContext() {
	m.pendingUndoAction = ""
	m.pendingUndoContext = ""
}

// saveAndRefresh saves the project and refreshes the tree view.
func (m *ProjectTreeModel) saveAndRefresh() tea.Cmd {
	// Capture undo context before returning the command
	action := m.pendingUndoAction
	context := m.pendingUndoContext
	undoDepth := 0
	if m.config.UndoHistoryDepth != nil {
		undoDepth = *m.config.UndoHistoryDepth
	}
	// Capture config references for closure
	cfg := m.config
	configManager := m.configManager

	// Clear the context for next operation
	m.clearUndoContext()

	return func() tea.Msg {
		// Check for auto-unblocking before save
		m.project.CheckAndAutoUnblock()

		// Generate recurring task instances for completed recurring tasks
		m.project.GenerateRecurringInstances()

		// Evaluate workflow rules
		rules := cfg.GetEnabledWorkflowRules()
		if len(rules) > 0 {
			result := workflow.EvaluateRules(m.project.Tasks, rules)
			// Log conflicts as warnings (they're also shown in status bar)
			if len(result.Conflicts) > 0 {
				// Conflicts are silently ignored in TUI - tasks are skipped
				// User can use 'tt workflow test' to debug
				_ = result.Conflicts
			}
			// Persist trigger counts if any rules fired
			if len(result.RuleTriggerCounts) > 0 && configManager != nil {
				cfg.IncrementRuleTriggerCounts(result.RuleTriggerCounts)
				// Ignore save errors for trigger counts - non-critical
				_ = configManager.Save(cfg)
			}
		}

		// Reorder root tasks so uncompleted tasks appear before completed tasks
		m.project.ReorderRootTasksByCompletion()

		// Record undo snapshot before saving (if undo is enabled and action is set)
		if undoDepth > 0 && action != "" {
			// Ignore errors - undo snapshot failures shouldn't block normal operation
			_ = storage.RecordUndoSnapshot(m.project, action, context, undoDepth)
		}

		// Create backup before saving for legacy undo support
		if err := storage.BackupProject(m.project); err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to create backup: %w", err)}
		}

		if err := storage.SaveProject(m.project); err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to save project: %w", err)}
		}
		storage.LoadProject(m.project)
		m.buildTree()
		return treeRefreshMsg{}
	}
}

// deleteTask removes a task and its children from the project.
func (m *ProjectTreeModel) deleteTask() tea.Cmd {
	// Capture config value before returning command
	undoDepth := 0
	if m.config.UndoHistoryDepth != nil {
		undoDepth = *m.config.UndoHistoryDepth
	}

	return func() tea.Msg {
		if m.deleteTarget == nil {
			return nil
		}

		task := m.deleteTarget
		m.deleteTarget = nil

		// Record undo snapshot before deleting
		if undoDepth > 0 {
			// Ignore errors - undo snapshot failures shouldn't block normal operation
			_ = storage.RecordUndoSnapshot(m.project, "Delete Task",
				fmt.Sprintf("#%d %s", task.ShortID, task.Title), undoDepth)
		}

		// Create backup before deleting for undo support
		if err := storage.BackupProject(m.project); err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to create backup: %w", err)}
		}

		// Collect IDs of task and all descendants to delete
		idsToDelete := make(map[string]bool)
		var collectIDs func(t *models.Task)
		collectIDs = func(t *models.Task) {
			idsToDelete[t.ID] = true
			for _, child := range t.Children {
				collectIDs(child)
			}
		}
		collectIDs(task)

		// Exit focus mode if focused task is being deleted
		if m.focusedTask != nil && idsToDelete[m.focusedTask.ID] {
			m.focusedTask = nil
		}

		// Filter out deleted tasks
		var remaining []*models.Task
		for _, t := range m.project.Tasks {
			if !idsToDelete[t.ID] {
				// Clear parent reference if parent was deleted
				if t.ParentID != nil && idsToDelete[*t.ParentID] {
					t.ParentID = nil
				}
				remaining = append(remaining, t)
			}
		}
		m.project.Tasks = remaining

		if err := storage.SaveProject(m.project); err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to save project: %w", err)}
		}

		storage.LoadProject(m.project)
		m.buildTree()
		return TaskDeletedMsg{Task: task}
	}
}

// bulkDeleteTasks removes all selected tasks and their children from the project.
func (m *ProjectTreeModel) bulkDeleteTasks() tea.Cmd {
	// Capture config value and count before returning command
	undoDepth := 0
	if m.config.UndoHistoryDepth != nil {
		undoDepth = *m.config.UndoHistoryDepth
	}
	taskCount := len(m.selectedTasks)

	return func() tea.Msg {
		if taskCount == 0 {
			return nil
		}

		// Record undo snapshot before deleting
		if undoDepth > 0 {
			// Ignore errors - undo snapshot failures shouldn't block normal operation
			_ = storage.RecordUndoSnapshot(m.project, "Delete Tasks",
				fmt.Sprintf("%d tasks", taskCount), undoDepth)
		}

		// Create backup before deleting for undo support
		if err := storage.BackupProject(m.project); err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to create backup: %w", err)}
		}

		// Collect all IDs to delete (selected tasks and their descendants)
		idsToDelete := make(map[string]bool)
		var collectIDs func(t *models.Task)
		collectIDs = func(t *models.Task) {
			idsToDelete[t.ID] = true
			for _, child := range t.Children {
				collectIDs(child)
			}
		}

		for _, task := range m.project.Tasks {
			if m.selectedTasks[task.ID] {
				collectIDs(task)
			}
		}

		// Exit focus mode if focused task is being deleted
		if m.focusedTask != nil && idsToDelete[m.focusedTask.ID] {
			m.focusedTask = nil
		}

		// Filter out deleted tasks
		var remaining []*models.Task
		for _, t := range m.project.Tasks {
			if !idsToDelete[t.ID] {
				// Clear parent reference if parent was deleted
				if t.ParentID != nil && idsToDelete[*t.ParentID] {
					t.ParentID = nil
				}
				remaining = append(remaining, t)
			}
		}
		m.project.Tasks = remaining

		if err := storage.SaveProject(m.project); err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to save project: %w", err)}
		}

		m.clearSelection()
		storage.LoadProject(m.project)
		m.buildTree()
		m.clampCursor()
		return treeRefreshMsg{}
	}
}

// applyBulkTag adds a tag to all selected tasks.
func (m *ProjectTreeModel) applyBulkTag(tag string) {
	for _, task := range m.getSelectedTasks() {
		task.AddTag(tag)
	}
}

// applyBulkChanges applies bulk edit changes to the given tasks.
func (m *ProjectTreeModel) applyBulkChanges(changes PendingChanges, tasks []*models.Task) tea.Cmd {
	if !changes.HasChanges() || len(tasks) == 0 {
		return nil
	}

	// Capture config value before returning command
	undoDepth := 0
	if m.config.UndoHistoryDepth != nil {
		undoDepth = *m.config.UndoHistoryDepth
	}
	taskCount := len(tasks)

	return func() tea.Msg {
		// Record undo snapshot before applying changes
		if undoDepth > 0 {
			_ = storage.RecordUndoSnapshot(m.project, "Bulk Edit",
				fmt.Sprintf("%d tasks", taskCount), undoDepth)
		}

		// Create backup for undo support
		if err := storage.BackupProject(m.project); err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to create backup: %w", err)}
		}

		// Apply changes to each task
		for _, task := range tasks {
			if changes.SetStatus != nil {
				task.Status = *changes.SetStatus
			}
			if changes.SetPriority != nil {
				task.Priority = *changes.SetPriority
			}
			for _, tag := range changes.AddTags {
				task.AddTag(tag)
			}
			for _, tag := range changes.RemoveTags {
				task.RemoveTag(tag)
			}
			if changes.SetSection != nil {
				task.Section = *changes.SetSection
			}
			if changes.ClearDueDate {
				task.DueDate = nil
			} else if changes.SetDueDate != nil {
				task.DueDate = changes.SetDueDate
			}
		}

		// Check for auto-unblocking
		m.project.CheckAndAutoUnblock()

		// Reorder root tasks
		m.project.ReorderRootTasksByCompletion()

		// Save the project
		if err := storage.SaveProject(m.project); err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to save project: %w", err)}
		}

		// Clear selection and rebuild tree
		m.clearSelection()
		storage.LoadProject(m.project)
		m.buildTree()
		m.clampCursor()

		return treeRefreshMsg{}
	}
}
