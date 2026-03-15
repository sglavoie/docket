package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/sglavoie/tt/internal/models"
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
	return performUndoCmd(m.projectService, m.project)
}

// buildWorkflowOpts constructs a WorkflowSaveOptions from the current model
// state, draining the pending undo context. Must be called at persist time
// (inside closures) so the undo context reflects the latest state.
func (m *ProjectTreeModel) buildWorkflowOpts(prevStates map[string]*models.Task) WorkflowSaveOptions {
	opts := BuildWorkflowOpts(m.projectService, m.project, m.config, m.configManager, prevStates, m.undo.Action, m.undo.Context)
	m.undo.Clear()
	return opts
}

// workflowPersistFn returns a persistFn that runs the full save pipeline
// used by ProjectTree: auto-unblock, recurring, workflow rules, undo snapshots.
func (m *ProjectTreeModel) workflowPersistFn() persistFn {
	return newWorkflowPersistFn(m)
}

// setUndoContext sets the action context for the next save operation.
func (m *ProjectTreeModel) setUndoContext(action, context string) {
	m.undo.Set(action, context)
}

// getMultiSelectedTasks returns selected tasks when in multi-select mode, nil otherwise.
func (m *ProjectTreeModel) getMultiSelectedTasks() []*models.Task {
	if m.multiSelectMode && len(m.selectedTasks) > 0 {
		return m.getSelectedTasks()
	}
	return nil
}

// clearMultiSelect exits multi-select mode and clears the selection.
func (m *ProjectTreeModel) clearMultiSelect() {
	m.clearSelection()
}

// refreshView rebuilds the tree display.
func (m *ProjectTreeModel) refreshView() {
	m.buildTree()
}

// saveAndRefreshBulk saves the project with workflow rules and rebuilds the tree.
func (m *ProjectTreeModel) saveAndRefreshBulk(prevStates map[string]*models.Task) tea.Cmd {
	return m.saveAndRefreshWithPrevStates(prevStates)
}

// setStatusMessage sets the status bar text.
func (m *ProjectTreeModel) setStatusMessage(msg string) {
	m.statusMessage = msg
}

// saveAndRefresh saves the project and refreshes the tree view.
func (m *ProjectTreeModel) saveAndRefresh() tea.Cmd {
	return m.saveAndRefreshWithPrevStates(nil)
}

// saveAndRefreshWithPrevStates saves the project with event trigger support.
// If prevStates is provided, event-based workflow triggers will be evaluated.
func (m *ProjectTreeModel) saveAndRefreshWithPrevStates(prevStates map[string]*models.Task) tea.Cmd {
	opts := m.buildWorkflowOpts(prevStates)

	return func() tea.Msg {
		if err := persistWithWorkflow(opts); err != nil {
			return ErrorMsg{Err: err}
		}
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
func buildEvaluationContext(tasks []*models.Task, prevStates map[string]*models.Task, projectName, projectID string) *workflow.EvaluationContext {
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
		Changes:     changes,
		AllTasks:    tasks,
		TaskMap:     taskMap,
		ProjectName: projectName,
		ProjectID:   projectID,
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

// showDeleteConfirm shows the appropriate delete confirmation for a single task.
func (m *ProjectTreeModel) showDeleteConfirm(task *models.Task) {
	m.deleteTarget = task
	if task.IsRecurring() {
		m.confirm.Show("delete-recurring",
			fmt.Sprintf("Delete recurring task #%d '%s'? (d)elete instance / (s)top recurrence / (n)o",
				task.ShortID, task.Title))
	} else {
		childCount := task.CountChildren()
		if childCount > 0 {
			m.confirm.Show("delete",
				fmt.Sprintf("Delete task #%d '%s' and %d children?", task.ShortID, task.Title, childCount))
		} else {
			m.confirm.Show("delete",
				fmt.Sprintf("Delete task #%d '%s'?", task.ShortID, task.Title))
		}
	}
}

// showBulkDeleteConfirm shows a confirmation for deleting multiple selected tasks.
func (m *ProjectTreeModel) showBulkDeleteConfirm() {
	totalChildren := 0
	for _, task := range m.getSelectedTasks() {
		totalChildren += task.CountChildren()
	}
	if totalChildren > 0 {
		m.confirm.Show("bulk-delete",
			fmt.Sprintf("Delete %d selected tasks and %d children?", len(m.selectedTasks), totalChildren))
	} else {
		m.confirm.Show("bulk-delete",
			fmt.Sprintf("Delete %d selected tasks?", len(m.selectedTasks)))
	}
}

// deleteTask removes a task and its children from the project.
func (m *ProjectTreeModel) deleteTask() tea.Cmd {
	if m.deleteTarget == nil {
		return nil
	}

	task := m.deleteTarget
	m.deleteTarget = nil

	m.setUndoContext("Delete Task", fmt.Sprintf("#%d %s", task.ShortID, task.Title))

	// Exit focus mode if focused task is being deleted
	if m.focusedTask != nil && m.focusedTask.ID == task.ID {
		m.focusedTask = nil
	}

	pf := m.workflowPersistFn()
	result := applyDeleteTask(pf, m.project, task.ID, m.config.DataDir)
	if result.Err != nil {
		return func() tea.Msg { return ErrorMsg{Err: result.Err} }
	}

	m.buildTree()
	return result.Cmd
}

// bulkDeleteTasks removes all selected tasks and their children from the project.
func (m *ProjectTreeModel) bulkDeleteTasks() tea.Cmd {
	taskCount := len(m.selectedTasks)
	if taskCount == 0 {
		return nil
	}

	m.setUndoContext("Delete Tasks", fmt.Sprintf("%d tasks", taskCount))

	// Exit focus mode if focused task is among those being deleted
	if m.focusedTask != nil && m.selectedTasks[m.focusedTask.ID] {
		m.focusedTask = nil
	}

	pf := m.workflowPersistFn()
	for taskID := range m.selectedTasks {
		result := applyDeleteTask(pf, m.project, taskID, m.config.DataDir)
		if result.Err != nil {
			return func() tea.Msg { return ErrorMsg{Err: result.Err} }
		}
	}

	m.clearSelection()
	m.buildTree()
	m.clampCursor()
	return func() tea.Msg { return treeRefreshMsg{} }
}

// applyBulkChanges applies bulk edit changes to the given tasks.
func (m *ProjectTreeModel) applyBulkChanges(changes PendingChanges, tasks []*models.Task) tea.Cmd {
	if !changes.HasChanges() || len(tasks) == 0 {
		return nil
	}

	m.setUndoContext("Bulk Edit", fmt.Sprintf("%d tasks", len(tasks)))

	return func() tea.Msg {
		// Capture task states before changes for event-based workflow triggers
		prevStates := captureTaskStates(m.project.Tasks)

		// Apply changes to each task
		typeSkipped := 0
		for _, task := range tasks {
			// Handle type change with hierarchy validation (outside MutateTask
			// since we need to check parent/children relationships)
			if changes.SetType != nil {
				newType := *changes.SetType
				canChange := true

				// Check if new type is valid under current parent
				if task.Parent != nil {
					if !newType.CanBeChildOf(task.Parent.Type) {
						canChange = false
					}
				}

				// Check if new type can have children (if task has any)
				if canChange && len(task.Children) > 0 && !newType.CanHaveChildren() {
					canChange = false
				}

				// Check if all children are valid under new type
				if canChange && len(task.Children) > 0 {
					for _, child := range task.Children {
						if !child.Type.CanBeChildOf(newType) {
							canChange = false
							break
						}
					}
				}

				if canChange {
					m.project.MutateTask(task.ID, func(t *models.Task) {
						t.Type = newType
						t.ClearTypeAttrs()
					})
				} else {
					typeSkipped++
				}
			}

			m.project.MutateTask(task.ID, func(t *models.Task) {
				if changes.SetStatus != nil {
					t.ApplyTransition(*changes.SetStatus)
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
				if changes.ClearRecurrence {
					t.ClearRecurrence()
				} else if changes.SetRecurrence != nil {
					t.Recurrence = &models.RecurrencePattern{
						Interval: changes.SetRecurrence.Interval,
						Unit:     changes.SetRecurrence.Unit,
						Weekdays: changes.SetRecurrence.Weekdays,
					}
				}
			})
			if changes.SetSection != nil {
				section := *changes.SetSection
				if models.IsGTDSection(section) || section == "" {
					m.project.RelocateTaskToEndOfSection(task.ID, section)
				}
			}
		}
		_ = typeSkipped // Available for future status message use

		// Handle reparenting (after other mutations, before persist)
		if changes.ClearParent || changes.SetParentID != nil {
			parentSkipped := 0
			for _, task := range tasks {
				if changes.ClearParent {
					m.project.MutateTask(task.ID, func(t *models.Task) {
						t.ParentID = nil
					})
				} else if changes.SetParentID != nil {
					targetID := *changes.SetParentID
					canReparent := true

					// Find target parent task
					var targetParent *models.Task
					for _, t := range m.project.Tasks {
						if t.ID == targetID {
							targetParent = t
							break
						}
					}
					if targetParent == nil {
						canReparent = false
					}

					// Validate type compatibility
					if canReparent && !task.Type.CanBeChildOf(targetParent.Type) {
						canReparent = false
					}

					// Circular dependency check: walk up from target parent,
					// if any ancestor is the task being moved, skip it
					if canReparent {
						current := targetParent
						for current != nil {
							if current.ID == task.ID {
								canReparent = false
								break
							}
							if current.ParentID == nil {
								break
							}
							// Find parent in project tasks
							var next *models.Task
							for _, t := range m.project.Tasks {
								if current.ParentID != nil && t.ID == *current.ParentID {
									next = t
									break
								}
							}
							current = next
						}
					}

					if canReparent {
						m.project.MutateTask(task.ID, func(t *models.Task) {
							t.ParentID = &targetID
						})
					} else {
						parentSkipped++
					}
				}
			}
			_ = parentSkipped // Available for future status message use
			m.project.BuildTaskTree()
		}

		opts := m.buildWorkflowOpts(prevStates)
		if err := persistWithWorkflow(opts); err != nil {
			return ErrorMsg{Err: err}
		}

		m.clearSelection()
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
