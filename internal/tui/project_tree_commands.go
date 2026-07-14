package tui

import (
	"fmt"
	"maps"
	"slices"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/workflow"
)

func (m *ProjectTreeModel) archiveProject() tea.Cmd {
	return projectArchiveRequestCmd(m.project, true)
}

func (m *ProjectTreeModel) unarchiveProject() tea.Cmd {
	return projectArchiveRequestCmd(m.project, false)
}

func projectArchiveRequestCmd(project *models.Project, archive bool) tea.Cmd {
	if project == nil {
		return nil
	}
	return func() tea.Msg {
		return ProjectArchiveRequestMsg{Project: project, Archive: archive}
	}
}

// performUndo restores the project from the most recent undo snapshot.
func (m *ProjectTreeModel) performUndo() tea.Cmd {
	return performUndoForProjectsCmd(m.projectService, m.project, activeUndoProjectsFromRuntime(m.runtime), repairRuntimeTaskRefsAfterRestore(m.runtime))
}

// performRedo restores the project from the most recent redo snapshot.
func (m *ProjectTreeModel) performRedo() tea.Cmd {
	return performRedoForProjectsCmd(m.projectService, m.project, activeUndoProjectsFromRuntime(m.runtime), repairRuntimeTaskRefsAfterRestore(m.runtime))
}

// commitWorkflowOpts constructs the project-scoped workflow options for the
// application-layer commit, draining the pending undo context. Must be called at
// persist time so the undo action/context reflect the latest state. Workflow rules
// come from config (nil-guarded for isolated render/unit tests); a nil prevStates
// selects stateless evaluation, a captured snapshot selects contextual evaluation.
func (m *ProjectTreeModel) commitWorkflowOpts(prevStates map[string]*models.Task) app.ProjectWorkflowOptions {
	var rules []workflow.WorkflowRule
	if m.runtime != nil {
		rules = m.runtime.Automation().GetEnabledWorkflowRules()
	}
	action, context := m.undo.Action, m.undo.Context
	m.undo.Clear()
	return app.ProjectWorkflowOptions{
		Rules:       rules,
		PrevStates:  prevStates,
		UndoAction:  action,
		UndoContext: context,
	}
}

// singleTaskCoordinator exposes the application-layer single-task coordinator so
// the shared primary-view helpers route single-task mutations through the
// coordinator (bulk paths remain on saveAndRefreshBulk).
func (m *ProjectTreeModel) singleTaskCoordinator() TaskItemCoordinator {
	return m.taskItem
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
// The view has already applied its mutation to m.project in memory; this commits
// that state through the application layer (workflow cascade + grouped ledger
// undo) and the shell reloads the tree on the resulting BatchTransactionResultMsg.
func (m *ProjectTreeModel) saveAndRefreshWithPrevStates(prevStates map[string]*models.Task) tea.Cmd {
	return m.taskItem.CommitProjectWorkflow(m.project, m.commitWorkflowOpts(prevStates), "")
}

// captureTaskStates creates a snapshot of task states for event trigger detection.
func captureTaskStates(tasks []*models.Task) map[string]*models.Task {
	return workflow.CaptureTaskStates(tasks)
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

	deleteIDs := collectDescendantIDs(m.project.Tasks, map[string]bool{task.ID: true})

	// Exit focus mode if focused task is being deleted
	if m.focusedTask != nil && m.focusedTask.ID == task.ID {
		m.focusedTask = nil
	}

	artifacts, err := m.deleteConfigArtifacts(deleteIDs)
	if err != nil {
		return func() tea.Msg { return ErrorMsg{Err: err} }
	}
	return m.bulk.Delete([]string{task.ID}, artifacts...)
}

// deleteConfigArtifacts folds the durable planning-ref cleanup (Focus Queue,
// Daily Plan, dismissals) for the removed subtree into the delete transaction as
// a config artifact, mirroring the cross-project outliner delete. Machine-local
// recent visits are repaired by the shell after the commit from RemovedTaskIDs.
func (m *ProjectTreeModel) deleteConfigArtifacts(deleteIDs map[string]bool) ([]storage.ArtifactChange, error) {
	if m.runtime == nil || m.configManager == nil {
		return nil, nil
	}
	planning := clonePlanningState(m.runtime.Planning())
	if planning.RemoveDurableTaskRefs(deleteIDs) == 0 {
		return nil, nil
	}
	return planningDomainArtifact(m.configManager, planning)
}

// bulkDeleteTasks removes all selected tasks and their children from the project.
func (m *ProjectTreeModel) bulkDeleteTasks() tea.Cmd {
	taskCount := len(m.selectedTasks)
	if taskCount == 0 {
		return nil
	}

	allDeleteIDs := collectDescendantIDs(m.project.Tasks, m.selectedTasks)

	// Exit focus mode if focused task is among those being deleted
	if m.focusedTask != nil && m.selectedTasks[m.focusedTask.ID] {
		m.focusedTask = nil
	}

	rootIDs := slices.Collect(maps.Keys(m.selectedTasks))

	artifacts, err := m.deleteConfigArtifacts(allDeleteIDs)
	if err != nil {
		return func() tea.Msg { return ErrorMsg{Err: err} }
	}
	m.clearSelection()
	return m.bulk.Delete(rootIDs, artifacts...)
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
	statusSkipped := 0
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

		// Gate the status transition outside MutateTask: skip it when the
		// transition is invalid or would complete a parent with incomplete
		// children, while still applying the other field changes below.
		applyStatus := false
		var targetState models.WorkflowState
		if changes.SetStatus != nil {
			targetState = models.TargetStateForStatus(*changes.SetStatus)
			if models.CanTransitionState(task.State(), targetState) == "" && (!targetState.IsTerminal() || !task.HasIncompleteChildren()) {
				applyStatus = true
			} else {
				statusSkipped++
			}
		}

		m.project.MutateTask(task.ID, func(t *models.Task) {
			if applyStatus {
				t.ApplyStateTransition(targetState)
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
	_ = typeSkipped   // Available for future status message use
	_ = statusSkipped // Available for future status message use

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

	opts := m.commitWorkflowOpts(prevStates)
	m.clearSelection()

	return m.taskItem.CommitProjectWorkflow(m.project, opts, "")
}

// archiveCompletedTasks moves all completed tasks to the archive file.
func (m *ProjectTreeModel) archiveCompletedTasks() tea.Cmd {
	// Capture config value before returning command
	undoDepth := 0
	if m.runtime.Settings().UndoHistoryDepth != nil {
		undoDepth = *m.runtime.Settings().UndoHistoryDepth
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
