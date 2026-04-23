package tui

import (
	"fmt"
	"maps"
	"slices"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/workflow"
)

func (m *ProjectTreeModel) archiveProject() tea.Cmd {
	// Mutate state synchronously to avoid data races with View()
	m.project.Archive()

	// Clear current project if it was the archived one
	if m.config.CurrentProject == m.project.ID {
		m.config.CurrentProject = ""
	}

	// Capture references for the goroutine (only I/O inside)
	cfg := m.config
	mgr := m.configManager
	proj := m.project
	return func() tea.Msg {
		if err := mgr.Save(cfg); err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to save: %w", err)}
		}

		return ProjectArchivedMsg{Project: proj}
	}
}

func (m *ProjectTreeModel) unarchiveProject() tea.Cmd {
	// Mutate state synchronously to avoid data races with View()
	m.project.Unarchive()

	cfg := m.config
	mgr := m.configManager
	proj := m.project
	return func() tea.Msg {
		if err := mgr.Save(cfg); err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to save: %w", err)}
		}

		return ProjectArchivedMsg{Project: proj}
	}
}

// performUndo restores the project from the most recent undo snapshot.
func (m *ProjectTreeModel) performUndo() tea.Cmd {
	return performUndoCmd(m.projectService, m.project)
}

// performRedo restores the project from the most recent redo snapshot.
func (m *ProjectTreeModel) performRedo() tea.Cmd {
	return performRedoCmd(m.projectService, m.project)
}

// buildWorkflowOpts constructs a WorkflowSaveOptions from the current model
// state, draining the pending undo context. Must be called at persist time
// (inside closures) so the undo context reflects the latest state.
func (m *ProjectTreeModel) buildWorkflowOpts(prevStates map[string]*models.Task) WorkflowSaveOptions {
	opts := BuildWorkflowOpts(m.projectService, m.project, m.config, m.configManager, prevStates, m.undo.Action, m.undo.Context, m.undo.TaskIDs)
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

// setUndoTaskIDs sets the targeted task IDs for the next undo snapshot.
func (m *ProjectTreeModel) setUndoTaskIDs(ids []string) {
	m.undo.TaskIDs = ids
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

// refreshView rebuilds the tree display and clamps the cursor so it
// cannot point past the last visible node after filtering changes.
func (m *ProjectTreeModel) refreshView() {
	m.buildTree()
	m.clampCursor()
}

// saveAndRefreshBulk saves the project with workflow rules and rebuilds the tree.
func (m *ProjectTreeModel) saveAndRefreshBulk(prevStates map[string]*models.Task) tea.Cmd {
	return m.saveAndRefreshWithPrevStates(prevStates)
}

// pasteFromClipboard reads the clipboard, creates a task in the project's
// Inbox, registers an undo snapshot, and saves.
func (m *ProjectTreeModel) pasteFromClipboard() tea.Cmd {
	task, err := newInboxTaskFromClipboard(m.project)
	if err != nil {
		return func() tea.Msg { return StatusMsg("Clipboard is empty") }
	}
	title := task.Title
	if len([]rune(title)) > 40 {
		title = string([]rune(title)[:37]) + "..."
	}
	m.setUndoContext("Paste Task", task.Title)
	m.undo.TaskIDs = []string{task.ID}
	m.setStatus(fmt.Sprintf("Created \"%s\" in Inbox", title))
	return m.saveAndRefresh()
}

// setStatusMessage sets the status bar text.
func (m *ProjectTreeModel) setStatusMessage(msg string) {
	m.setStatus(msg)
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
		// Note: m.buildTree() is called by the Update handler on treeRefreshMsg
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

	// Copy recurrence if present
	var recurrenceCopy *models.RecurrencePattern
	if task.Recurrence != nil {
		recurrenceCopy = &models.RecurrencePattern{
			Interval:    task.Recurrence.Interval,
			Unit:        task.Recurrence.Unit,
			Skipped:     task.Recurrence.Skipped,
			Streak:      task.Recurrence.Streak,
			MaxStreak:   task.Recurrence.MaxStreak,
			OriginalDay: task.Recurrence.OriginalDay,
			Mode:        task.Recurrence.Mode,
		}
		if len(task.Recurrence.Weekdays) > 0 {
			recurrenceCopy.Weekdays = make([]int, len(task.Recurrence.Weekdays))
			copy(recurrenceCopy.Weekdays, task.Recurrence.Weekdays)
		}
	}

	return &models.Task{
		ID:            task.ID,
		Status:        task.Status,
		Priority:      task.Priority,
		Section:       task.Section,
		CustomSection: task.CustomSection,
		Tags:          tagsCopy,
		Type:          task.Type,
		DueDate:       task.DueDate,
		Description:   task.Description,
		Recurrence:    recurrenceCopy,
		// Preserve parent/child relationships for hierarchical triggers
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
		current.CustomSection != prev.CustomSection ||
		current.Type != prev.Type {
		return true
	}
	// Detect due date changes (added, removed, or modified)
	switch {
	case current.DueDate == nil && prev.DueDate == nil:
		// no change
	case current.DueDate == nil || prev.DueDate == nil:
		return true // one is nil, the other is not
	default:
		if !current.DueDate.Equal(*prev.DueDate) {
			return true
		}
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

// showBulkDeleteConfirm shows a confirmation for deleting multiple selected tasks.
func (m *ProjectTreeModel) showBulkDeleteConfirm() {
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

// deleteTask removes a task and its children from the project.
func (m *ProjectTreeModel) deleteTask() tea.Cmd {
	if m.deleteTarget == nil {
		return nil
	}

	task := m.deleteTarget
	m.deleteTarget = nil

	m.setUndoContext("Delete Task", fmt.Sprintf("#%d %s", task.ShortID, task.Title))
	deleteIDs := collectDescendantIDs(m.project.Tasks, map[string]bool{task.ID: true})
	m.setUndoTaskIDs(slices.Collect(maps.Keys(deleteIDs)))

	// Exit focus mode if focused task is being deleted
	if m.focusedTask != nil && m.focusedTask.ID == task.ID {
		m.focusedTask = nil
	}

	pf := m.workflowPersistFn()
	result := applyDeleteTask(pf, m.projectService, m.project, task.ID)
	if result.Err != nil {
		return func() tea.Msg { return ErrorMsg{Err: result.Err} }
	}

	// Clean up FocusQueue, DailyPlan, and RecentTaskVisits references.
	cleanupConfigTaskRefs(m.config, m.configManager, deleteIDs)

	m.buildTree()
	m.clampCursor()
	return result.Cmd
}

// bulkDeleteTasks removes all selected tasks and their children from the project.
func (m *ProjectTreeModel) bulkDeleteTasks() tea.Cmd {
	taskCount := len(m.selectedTasks)
	if taskCount == 0 {
		return nil
	}

	m.setUndoContext("Delete Tasks", fmt.Sprintf("%d tasks", taskCount))
	allDeleteIDs := collectDescendantIDs(m.project.Tasks, m.selectedTasks)
	m.setUndoTaskIDs(slices.Collect(maps.Keys(allDeleteIDs)))

	// Exit focus mode if focused task is among those being deleted
	if m.focusedTask != nil && m.selectedTasks[m.focusedTask.ID] {
		m.focusedTask = nil
	}

	pf := m.workflowPersistFn()
	for taskID := range m.selectedTasks {
		result := applyDeleteTask(pf, m.projectService, m.project, taskID)
		if result.Err != nil {
			return func() tea.Msg { return ErrorMsg{Err: result.Err} }
		}
	}

	// Clean up FocusQueue, DailyPlan, and RecentTaskVisits references.
	cleanupConfigTaskRefs(m.config, m.configManager, allDeleteIDs)

	m.clearSelection()
	m.buildTree()
	m.clampCursor()
	return func() tea.Msg { return treeRefreshMsg{} }
}

// applyBulkChanges applies bulk edit changes to the given tasks.
func (m *ProjectTreeModel) applyBulkChanges(changes PendingChanges, tasks []*models.Task) tea.Cmd { //nolint:gocyclo
	if !changes.HasChanges() || len(tasks) == 0 {
		return nil
	}

	m.setUndoContext("Bulk Edit", fmt.Sprintf("%d tasks", len(tasks)))

	// All mutations run synchronously to avoid data races with View()
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
				t.SetSection(*changes.SetSection)
			}
			if changes.ClearDueDate {
				t.DueDate = nil
				t.HasTime = false
			} else if changes.SetDueDate != nil {
				t.DueDate = changes.SetDueDate
				t.HasTime = false // Bulk edit always sets date-only values.
			}
			if changes.ClearRecurrence {
				t.ClearRecurrence()
			} else if changes.SetRecurrence != nil {
				// Use SetRecurrence to populate OriginalDay for monthly patterns
				_ = t.SetRecurrence(&models.RecurrencePattern{
					Interval: changes.SetRecurrence.Interval,
					Unit:     changes.SetRecurrence.Unit,
					Weekdays: changes.SetRecurrence.Weekdays,
				})
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
				taskMap := make(map[string]*models.Task, len(m.project.Tasks))
				for _, t := range m.project.Tasks {
					taskMap[t.ID] = t
					if t.ID == targetID {
						targetParent = t
					}
				}
				if targetParent == nil {
					canReparent = false
				}

				// Validate type compatibility
				if canReparent && !task.Type.CanBeChildOf(targetParent.Type) {
					canReparent = false
				}

				// Cycle check using shared utility
				if canReparent && models.WouldCreateCycle(taskMap, task.ID, targetID) {
					canReparent = false
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
	m.clearSelection()

	// Only I/O in the goroutine
	return func() tea.Msg {
		if err := persistWithWorkflow(opts); err != nil {
			return ErrorMsg{Err: err}
		}

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

	if completedCount == 0 {
		return nil
	}

	ps := m.projectService
	proj := m.project
	return func() tea.Msg {
		// Record undo snapshot before archiving
		if undoDepth > 0 {
			_ = ps.RecordUndoSnapshot(proj, "Archive Tasks",
				fmt.Sprintf("%d tasks", completedCount), undoDepth, nil)
		}

		// Create backup before archiving
		if err := ps.Backup(proj); err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to create backup: %w", err)}
		}

		// Archive the tasks
		count, err := ps.ArchiveTasks(proj)
		if err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to archive tasks: %w", err)}
		}

		// Reload project (tree rebuild happens in TasksArchivedMsg handler)
		_ = ps.Load(proj)

		return TasksArchivedMsg{Count: count}
	}
}
