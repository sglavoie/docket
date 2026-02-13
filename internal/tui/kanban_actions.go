package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/sglavoie/tt/internal/models"
)

// KanbanTaskMovedMsg signals that a task was moved to a different column.
type KanbanTaskMovedMsg struct {
	Task              *models.Task
	TargetColumnIndex int
	TargetSection     string
}

// KanbanTaskReorderedMsg signals that a task was reordered within its column.
type KanbanTaskReorderedMsg struct {
	Task        *models.Task
	Section     string
	NewPosition int // New cursor position after reorder
}

// KanbanStatusChangedMsg signals that a task's status was changed.
// The task may have moved to a different column if it became completed/canceled.
type KanbanStatusChangedMsg struct {
	TaskID          string
	PreviousSection string
}

// buildWorkflowOpts constructs a WorkflowSaveOptions from the current Kanban
// model state. Must be called at persist time (inside closures).
func (m *KanbanViewModel) buildWorkflowOpts(prevStates map[string]*models.Task) WorkflowSaveOptions {
	return WorkflowSaveOptions{
		Persister:     defaultPersister,
		Project:       m.Project,
		Config:        m.config,
		ConfigManager: m.configManager,
		PrevStates:    prevStates,
	}
}

// workflowPersistFn returns a persistFn that runs the full save pipeline
// used by Kanban: auto-unblock, recurring, workflow rules.
func (m *KanbanViewModel) workflowPersistFn() persistFn {
	return workflowPersist(func() WorkflowSaveOptions {
		return m.buildWorkflowOpts(nil)
	})
}

// getSelectedTask returns the currently selected task, or nil if none.
func (m *KanbanViewModel) getSelectedTask() *models.Task {
	visibleNodes := m.currentVisibleNodes()
	cs := m.CurrentColumnState()

	if len(visibleNodes) == 0 || !cs.InBounds(len(visibleNodes)) {
		return nil
	}

	return visibleNodes[cs.Cursor].Task
}

// getSelectedNode returns the currently selected KanbanNode, or nil if none.
func (m *KanbanViewModel) getSelectedNode() *KanbanNode {
	visibleNodes := m.currentVisibleNodes()
	cs := m.CurrentColumnState()

	if len(visibleNodes) == 0 || !cs.InBounds(len(visibleNodes)) {
		return nil
	}

	return visibleNodes[cs.Cursor]
}

// saveWithWorkflow saves the project with the full workflow pipeline and returns
// a command that produces the given message on success. This is the shared
// implementation used by all Kanban save operations.
func (m *KanbanViewModel) saveWithWorkflow(prevStates map[string]*models.Task, makeMsg func() tea.Msg) tea.Cmd {
	opts := m.buildWorkflowOpts(prevStates)

	return func() tea.Msg {
		if err := persistWithWorkflow(opts); err != nil {
			return ErrorMsg{Err: err}
		}
		return makeMsg()
	}
}

// moveTaskLeft moves the currently selected task to the column on the left.
// Returns a command that persists the change with workflow rule evaluation.
// If the task is already in the leftmost column, the operation is ignored.
func (m *KanbanViewModel) moveTaskLeft() tea.Cmd {
	// Get current column and check if we're at leftmost
	if m.ActiveColumn <= 0 {
		return nil
	}

	task := m.getSelectedTask()
	if task == nil {
		return nil
	}

	targetColumnIndex := m.ActiveColumn - 1
	targetKey := m.columnDefs[targetColumnIndex].Key

	prevStates := captureTaskStates(m.Project.Tasks)

	if !m.Project.MutateTask(task.ID, func(t *models.Task) {
		ApplyColumnMove(m.groupBy, t, targetKey)
	}) {
		return nil
	}
	if IsSectionGroupBy(m.groupBy) {
		m.Project.RelocateTaskToEndOfSection(task.ID, targetKey)
	}

	return m.saveWithWorkflowRules(prevStates, task, targetColumnIndex, targetKey)
}

// saveWithWorkflowRules saves the project with workflow rule evaluation,
// then updates the Kanban view state to reflect the changes.
func (m *KanbanViewModel) saveWithWorkflowRules(prevStates map[string]*models.Task, task *models.Task, targetColumnIndex int, targetSection string) tea.Cmd {
	return m.saveWithWorkflow(prevStates, func() tea.Msg {
		return KanbanTaskMovedMsg{
			Task:              task,
			TargetColumnIndex: targetColumnIndex,
			TargetSection:     targetSection,
		}
	})
}

// moveTaskRight moves the currently selected task to the column on the right.
// Returns a command that persists the change with workflow rule evaluation.
// If the task is already in the rightmost column, the operation is ignored.
func (m *KanbanViewModel) moveTaskRight() tea.Cmd {
	if m.ActiveColumn >= len(m.columnDefs)-1 {
		return nil
	}

	task := m.getSelectedTask()
	if task == nil {
		return nil
	}

	targetColumnIndex := m.ActiveColumn + 1
	targetKey := m.columnDefs[targetColumnIndex].Key

	prevStates := captureTaskStates(m.Project.Tasks)

	if !m.Project.MutateTask(task.ID, func(t *models.Task) {
		ApplyColumnMove(m.groupBy, t, targetKey)
	}) {
		return nil
	}
	if IsSectionGroupBy(m.groupBy) {
		m.Project.RelocateTaskToEndOfSection(task.ID, targetKey)
	}

	return m.saveWithWorkflowRules(prevStates, task, targetColumnIndex, targetKey)
}

// swapTaskDown swaps the currently selected task with the task below it.
// Returns a command that persists the change with workflow rule evaluation.
// If the task is at the bottom of the column, the operation is ignored.
func (m *KanbanViewModel) swapTaskDown() tea.Cmd {
	currentKey := m.activeColumnKey()
	visibleNodes := m.visibleNodesForSection(currentKey)
	cs := m.Columns[currentKey]

	// Check bounds: need at least 2 visible nodes and cursor not at bottom
	if len(visibleNodes) < 2 || cs.Cursor >= len(visibleNodes)-1 {
		return nil
	}

	task := visibleNodes[cs.Cursor].Task
	taskBelow := visibleNodes[cs.Cursor+1].Task
	if task == nil || taskBelow == nil {
		return nil
	}

	// Capture previous task states for workflow rule evaluation
	prevStates := captureTaskStates(m.Project.Tasks)

	// Swap the tasks using project's SwapTasks method
	m.Project.SwapTasks(task, taskBelow)

	// New cursor position follows the task down
	newPosition := cs.Cursor + 1

	// Persist with workflow rule evaluation
	return m.saveWithWorkflowRulesForReorder(prevStates, task, currentKey, newPosition)
}

// swapTaskUp swaps the currently selected task with the task above it.
// Returns a command that persists the change with workflow rule evaluation.
// If the task is at the top of the column, the operation is ignored.
func (m *KanbanViewModel) swapTaskUp() tea.Cmd {
	currentKey := m.activeColumnKey()
	visibleNodes := m.visibleNodesForSection(currentKey)
	cs := m.Columns[currentKey]

	// Check bounds: need at least 2 visible nodes and cursor not at top
	if len(visibleNodes) < 2 || cs.Cursor <= 0 {
		return nil
	}

	task := visibleNodes[cs.Cursor].Task
	taskAbove := visibleNodes[cs.Cursor-1].Task
	if task == nil || taskAbove == nil {
		return nil
	}

	prevStates := captureTaskStates(m.Project.Tasks)
	m.Project.SwapTasks(task, taskAbove)
	newPosition := cs.Cursor - 1

	return m.saveWithWorkflowRulesForReorder(prevStates, task, currentKey, newPosition)
}

// moveTaskToTop moves the selected task to the top of the current column
// by repeatedly swapping up, matching the proven K (swap-up) mechanism.
func (m *KanbanViewModel) moveTaskToTop() tea.Cmd {
	currentKey := m.activeColumnKey()
	visibleNodes := m.visibleNodesForSection(currentKey)
	cs := m.Columns[currentKey]

	if len(visibleNodes) < 2 || cs.Cursor <= 0 {
		return nil
	}

	task := visibleNodes[cs.Cursor].Task
	if task == nil {
		return nil
	}

	prevStates := captureTaskStates(m.Project.Tasks)

	// Swap up through each visible node, same as repeated K presses
	for i := cs.Cursor - 1; i >= 0; i-- {
		target := visibleNodes[i].Task
		if target == nil {
			break
		}
		m.Project.SwapTasks(task, target)
	}

	cs.Cursor = 0
	return m.saveWithWorkflowRulesForReorder(prevStates, task, currentKey, 0)
}

// moveTaskToBottom moves the selected task to the bottom of the current column
// by repeatedly swapping down, matching the proven J (swap-down) mechanism.
func (m *KanbanViewModel) moveTaskToBottom() tea.Cmd {
	currentKey := m.activeColumnKey()
	visibleNodes := m.visibleNodesForSection(currentKey)
	cs := m.Columns[currentKey]

	lastIdx := len(visibleNodes) - 1
	if len(visibleNodes) < 2 || cs.Cursor >= lastIdx {
		return nil
	}

	task := visibleNodes[cs.Cursor].Task
	if task == nil {
		return nil
	}

	prevStates := captureTaskStates(m.Project.Tasks)

	// Swap down through each visible node, same as repeated J presses
	for i := cs.Cursor + 1; i <= lastIdx; i++ {
		target := visibleNodes[i].Task
		if target == nil {
			break
		}
		m.Project.SwapTasks(task, target)
	}

	cs.Cursor = lastIdx
	return m.saveWithWorkflowRulesForReorder(prevStates, task, currentKey, lastIdx)
}

// saveWithWorkflowRulesForReorder saves the project with workflow rule evaluation
// for within-column reordering operations.
func (m *KanbanViewModel) saveWithWorkflowRulesForReorder(prevStates map[string]*models.Task, task *models.Task, section string, newPosition int) tea.Cmd {
	return m.saveWithWorkflow(prevStates, func() tea.Msg {
		return KanbanTaskReorderedMsg{
			Task:        task,
			Section:     section,
			NewPosition: newPosition,
		}
	})
}

// KanbanUndoPerformedMsg signals that an undo was performed from within the kanban view.
// Unlike UndoPerformedMsg (which pops the view via the app handler), this message is
// handled entirely within the kanban model so the user stays in the kanban view.
type KanbanUndoPerformedMsg struct {
	Action string
}

// KanbanTaskDeletedMsg signals that a task was deleted from the Kanban view.
type KanbanTaskDeletedMsg struct {
	TaskID  string
	Section string
}

// executeDelete deletes the currently targeted task and its descendants.
func (m *KanbanViewModel) executeDelete() tea.Cmd {
	if m.deleteTarget == nil {
		return nil
	}

	task := m.deleteTarget
	section := task.Section
	m.deleteTarget = nil

	return func() tea.Msg {
		if err := defaultPersister.Backup(m.Project); err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to create backup: %w", err)}
		}

		if err := deleteTasksFromProject(m.Project, defaultPersister, map[string]bool{task.ID: true}, nil); err != nil {
			return ErrorMsg{Err: err}
		}

		return KanbanTaskDeletedMsg{
			TaskID:  task.ID,
			Section: section,
		}
	}
}

// performUndo restores the project from backup.
func (m *KanbanViewModel) performUndo() tea.Cmd {
	return func() tea.Msg {
		if err := performUndoFromBackup(defaultPersister, m.Project); err != nil {
			return ErrorMsg{Err: err}
		}
		m.loadTasks()
		return KanbanUndoPerformedMsg{Action: "Undo"}
	}
}

// sendTaskToSection moves the selected task to the specified section.
// Only operates in section group-by mode.
func (m *KanbanViewModel) sendTaskToSection(targetSection string) tea.Cmd {
	if !IsSectionGroupBy(m.groupBy) {
		return nil
	}

	task := m.getSelectedTask()
	if task == nil {
		return nil
	}

	targetColumnIndex := m.columnIndexForKey(targetSection)
	if targetColumnIndex < 0 {
		targetColumnIndex = 0
	}

	prevStates := captureTaskStates(m.Project.Tasks)
	if !m.Project.MutateTask(task.ID, func(t *models.Task) {
		t.Section = targetSection
	}) {
		return nil
	}
	m.Project.RelocateTaskToEndOfSection(task.ID, targetSection)

	m.statusMessage = fmt.Sprintf("Moved to %s", sectionDisplayName(targetSection))
	return m.saveWithWorkflowRules(prevStates, task, targetColumnIndex, targetSection)
}

// indentTask makes the selected task a child of a valid parent above it.
// Returns a command that persists the change with workflow rule evaluation.
// If no valid parent is found, the operation is silently ignored.
func (m *KanbanViewModel) indentTask() tea.Cmd {
	visibleNodes := m.currentVisibleNodes()
	cs := m.CurrentColumnState()

	// Need cursor > 0 to have a potential parent above
	if len(visibleNodes) == 0 || cs.Cursor <= 0 {
		return nil
	}

	task := visibleNodes[cs.Cursor].Task
	if task == nil {
		return nil
	}

	// Re-lookup task to avoid stale pointer after concurrent reload
	current := m.Project.GetTaskByID(task.ID)
	if current == nil {
		return nil
	}

	// Walk up from the node above to find a valid parent
	candidate := visibleNodes[cs.Cursor-1].Task
	for candidate != nil {
		if ok, _ := current.CanIndent(candidate); ok {
			prevStates := captureTaskStates(m.Project.Tasks)
			candidateID := candidate.ID
			m.Project.MutateTask(task.ID, func(t *models.Task) {
				t.ParentID = &candidateID
			})
			m.Project.BuildTaskTree()

			return m.saveWithWorkflowRulesForReorder(prevStates, task, m.activeColumnKey(), cs.Cursor)
		}
		candidate = candidate.Parent
	}

	return nil
}

// outdentTask moves the selected task to be a sibling of its current parent.
// Returns a command that persists the change with workflow rule evaluation.
// If the task cannot be outdented, the operation is silently ignored.
func (m *KanbanViewModel) outdentTask() tea.Cmd {
	task := m.getSelectedTask()
	if task == nil {
		return nil
	}

	// Check if outdent is valid
	if ok, _ := task.CanOutdent(); !ok {
		return nil
	}

	prevStates := captureTaskStates(m.Project.Tasks)
	_ = m.Project.OutdentTask(task)

	cs := m.CurrentColumnState()
	return m.saveWithWorkflowRulesForReorder(prevStates, task, m.activeColumnKey(), cs.Cursor)
}

// executeArchiveToggle archives or unarchives the project.
func (m *KanbanViewModel) executeArchiveToggle() tea.Cmd {
	if m.Project.Sandbox {
		m.statusMessage = "Cannot archive the sandbox project"
		return nil
	}

	prevStates := captureTaskStates(m.Project.Tasks)

	if m.Project.Archived {
		m.Project.Unarchive()
		m.statusMessage = "Project unarchived"
	} else {
		m.Project.Archive()
		m.statusMessage = "Project archived"
	}

	return m.saveWithWorkflow(prevStates, func() tea.Msg {
		return KanbanStatusChangedMsg{TaskID: "", PreviousSection: ""}
	})
}
