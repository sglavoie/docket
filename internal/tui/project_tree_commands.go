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
		if !m.projectService.BackupExists(m.project) {
			return ErrorMsg{Err: fmt.Errorf("no backup available to undo")}
		}

		tasksBefore := len(m.project.Tasks)

		if err := m.projectService.RestoreFromBackup(m.project); err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to undo: %w", err)}
		}

		if err := m.projectService.Load(m.project); err != nil {
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
	return m.saveAndRefreshWithPrevStates(nil)
}

// saveAndRefreshWithPrevStates saves the project with event trigger support.
// If prevStates is provided, event-based workflow triggers will be evaluated.
func (m *ProjectTreeModel) saveAndRefreshWithPrevStates(prevStates map[string]*models.Task) tea.Cmd {
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
			var result *workflow.EvaluationResult
			if prevStates != nil {
				// Use context-aware evaluation with event triggers
				evalCtx := buildEvaluationContext(m.project.Tasks, prevStates)
				result = workflow.EvaluateRulesWithContext(evalCtx, rules)
			} else {
				result = workflow.EvaluateRules(m.project.Tasks, rules)
			}
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
			_ = m.projectService.RecordUndoSnapshot(m.project, action, context, undoDepth)
		}

		// Create backup before saving for legacy undo support
		if err := m.projectService.Backup(m.project); err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to create backup: %w", err)}
		}

		if err := m.projectService.Save(m.project); err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to save project: %w", err)}
		}
		_ = m.projectService.Load(m.project)
		m.buildTree()
		return treeRefreshMsg{}
	}
}

// captureTaskStates creates a snapshot of task states for event trigger detection.
func captureTaskStates(tasks []*models.Task) map[string]*models.Task {
	states := make(map[string]*models.Task, len(tasks))
	for _, task := range tasks {
		states[task.ID] = copyTaskState(task)
	}
	return states
}

// copyTaskState creates a shallow copy of task fields relevant for event detection.
func copyTaskState(task *models.Task) *models.Task {
	if task == nil {
		return nil
	}
	tagsCopy := make([]string, len(task.Tags))
	copy(tagsCopy, task.Tags)

	return &models.Task{
		ID:       task.ID,
		Status:   task.Status,
		Priority: task.Priority,
		Section:  task.Section,
		Tags:     tagsCopy,
		Type:     task.Type,
		Parent:   task.Parent,
		Children: task.Children,
		Level:    task.Level,
	}
}

// buildEvaluationContext creates an evaluation context for workflow rules.
// Only tasks that actually changed are included in Changes to avoid excessive rule triggering.
// AllTasks and TaskMap still contain all tasks for hierarchical trigger lookups.
func buildEvaluationContext(tasks []*models.Task, prevStates map[string]*models.Task) *workflow.EvaluationContext {
	taskMap := make(map[string]*models.Task, len(tasks))
	var changes []workflow.TaskChangeContext

	for _, task := range tasks {
		taskMap[task.ID] = task
		prev := prevStates[task.ID]
		if prev == nil || taskStateChanged(task, prev) {
			changes = append(changes, workflow.TaskChangeContext{
				Task:     task,
				Previous: prev,
			})
		}
	}

	return &workflow.EvaluationContext{
		Changes:  changes,
		AllTasks: tasks,
		TaskMap:  taskMap,
	}
}

// taskStateChanged returns true if the task's workflow-relevant state changed.
func taskStateChanged(current, prev *models.Task) bool {
	if prev == nil {
		return true
	}
	if current.Status != prev.Status ||
		current.Priority != prev.Priority ||
		current.Section != prev.Section ||
		current.Type != prev.Type {
		return true
	}
	if len(current.Tags) != len(prev.Tags) {
		return true
	}
	for i, tag := range current.Tags {
		if tag != prev.Tags[i] {
			return true
		}
	}
	return false
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
			_ = m.projectService.RecordUndoSnapshot(m.project, "Delete Task",
				fmt.Sprintf("#%d %s", task.ShortID, task.Title), undoDepth)
		}

		// Create backup before deleting for undo support
		if err := m.projectService.Backup(m.project); err != nil {
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

		// Clean up attachments for all tasks being deleted
		for _, t := range m.project.Tasks {
			if idsToDelete[t.ID] {
				_ = storage.CleanupTaskAttachments(m.project, t)
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

		if err := m.projectService.Save(m.project); err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to save project: %w", err)}
		}

		_ = m.projectService.Load(m.project)
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
			_ = m.projectService.RecordUndoSnapshot(m.project, "Delete Tasks",
				fmt.Sprintf("%d tasks", taskCount), undoDepth)
		}

		// Create backup before deleting for undo support
		if err := m.projectService.Backup(m.project); err != nil {
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

		// Clean up attachments for all tasks being deleted
		for _, t := range m.project.Tasks {
			if idsToDelete[t.ID] {
				_ = storage.CleanupTaskAttachments(m.project, t)
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

		if err := m.projectService.Save(m.project); err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to save project: %w", err)}
		}

		m.clearSelection()
		_ = m.projectService.Load(m.project)
		m.buildTree()
		m.clampCursor()
		return treeRefreshMsg{}
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
	cfg := m.config
	configManager := m.configManager

	return func() tea.Msg {
		// Record undo snapshot before applying changes
		if undoDepth > 0 {
			_ = m.projectService.RecordUndoSnapshot(m.project, "Bulk Edit",
				fmt.Sprintf("%d tasks", taskCount), undoDepth)
		}

		// Create backup for undo support
		if err := m.projectService.Backup(m.project); err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to create backup: %w", err)}
		}

		// Capture task states before changes for event-based workflow triggers
		prevStates := captureTaskStates(m.project.Tasks)

		// Apply changes to each task
		for _, task := range tasks {
			m.project.MutateTask(task.ID, func(t *models.Task) {
				if changes.SetStatus != nil {
					t.Status = *changes.SetStatus
				}
				if changes.SetPriority != nil {
					t.Priority = *changes.SetPriority
				}
				for _, tag := range changes.AddTags {
					t.AddTag(tag)
				}
				for _, tag := range changes.RemoveTags {
					t.RemoveTag(tag)
				}
				if changes.SetSection != nil {
					if models.IsGTDSection(*changes.SetSection) || *changes.SetSection == "" {
						t.Section = *changes.SetSection
					} else {
						t.CustomSection = *changes.SetSection
					}
				}
				if changes.ClearDueDate {
					t.DueDate = nil
				} else if changes.SetDueDate != nil {
					t.DueDate = changes.SetDueDate
				}
			})
			if changes.SetSection != nil {
				section := *changes.SetSection
				if models.IsGTDSection(section) || section == "" {
					m.project.RelocateTaskToEndOfSection(task.ID, section)
				}
			}
		}

		// Check for auto-unblocking
		m.project.CheckAndAutoUnblock()

		// Evaluate workflow rules with event triggers
		rules := cfg.GetEnabledWorkflowRules()
		if len(rules) > 0 {
			evalCtx := buildEvaluationContext(m.project.Tasks, prevStates)
			result := workflow.EvaluateRulesWithContext(evalCtx, rules)
			if len(result.RuleTriggerCounts) > 0 && configManager != nil {
				cfg.IncrementRuleTriggerCounts(result.RuleTriggerCounts)
				_ = configManager.Save(cfg)
			}
		}

		// Reorder root tasks
		m.project.ReorderRootTasksByCompletion()

		// Save the project
		if err := m.projectService.Save(m.project); err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to save project: %w", err)}
		}

		// Clear selection and rebuild tree
		m.clearSelection()
		_ = m.projectService.Load(m.project)
		m.buildTree()
		m.clampCursor()

		return treeRefreshMsg{}
	}
}

// archiveCompletedTasks moves all completed tasks to the archive file.
func (m *ProjectTreeModel) archiveCompletedTasks() tea.Cmd {
	// Capture config value before returning command
	undoDepth := 0
	if m.config.UndoHistoryDepth != nil {
		undoDepth = *m.config.UndoHistoryDepth
	}
	completedCount := m.project.CountCompletedTasks()

	return func() tea.Msg {
		if completedCount == 0 {
			return nil
		}

		// Record undo snapshot before archiving
		if undoDepth > 0 {
			_ = m.projectService.RecordUndoSnapshot(m.project, "Archive Tasks",
				fmt.Sprintf("%d tasks", completedCount), undoDepth)
		}

		// Create backup before archiving
		if err := m.projectService.Backup(m.project); err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to create backup: %w", err)}
		}

		// Archive the tasks
		count, err := m.projectService.ArchiveTasks(m.project)
		if err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to archive tasks: %w", err)}
		}

		// Reload project and rebuild tree
		_ = m.projectService.Load(m.project)
		m.buildTree()
		m.clampCursor()

		return TasksArchivedMsg{Count: count}
	}
}
