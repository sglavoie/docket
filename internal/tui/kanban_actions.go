package tui

import (
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/workflow"
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
// The task may have moved to a different column if it became completed/cancelled.
type KanbanStatusChangedMsg struct {
	TaskID          string
	PreviousSection string
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

// saveWithWorkflow saves the project with workflow rule evaluation and returns
// a command that produces the given message on success. This is the shared
// implementation used by all Kanban save operations.
func (m *KanbanViewModel) saveWithWorkflow(prevStates map[string]*models.Task, makeMsg func() tea.Msg) tea.Cmd {
	cfg := m.config
	configManager := m.configManager

	return func() tea.Msg {
		// Backup before saving
		if err := storage.BackupProject(m.Project); err != nil {
			return ErrorMsg{Err: fmt.Errorf("backup failed: %w", err)}
		}

		// Evaluate workflow rules
		if cfg != nil {
			rules := cfg.GetEnabledWorkflowRules()
			if len(rules) > 0 {
				evalCtx := buildEvaluationContext(m.Project.Tasks, prevStates)
				result := workflow.EvaluateRulesWithContext(evalCtx, rules)
				if len(result.RuleTriggerCounts) > 0 && configManager != nil {
					cfg.IncrementRuleTriggerCounts(result.RuleTriggerCounts)
					_ = configManager.Save(cfg)
				}
			}
		}

		// Save the project
		if err := storage.SaveProject(m.Project); err != nil {
			return ErrorMsg{Err: fmt.Errorf("save failed: %w", err)}
		}

		// Reload project to ensure consistency
		_ = storage.LoadProject(m.Project)

		return makeMsg()
	}
}

// moveTaskLeft moves the currently selected task to the column on the left.
// Returns a command that persists the change with workflow rule evaluation.
// If the task is already in the leftmost column, the operation is ignored.
func (m *KanbanViewModel) moveTaskLeft() tea.Cmd {
	// Get current column and check if we're at leftmost
	if m.ActiveColumn <= 0 {
		// Already at leftmost column, ignore
		return nil
	}

	// Get the selected task via visible nodes
	task := m.getSelectedTask()
	if task == nil {
		return nil
	}

	// Determine target column (one to the left)
	targetColumnIndex := m.ActiveColumn - 1
	targetSection := KanbanColumnOrder[targetColumnIndex]

	// Capture previous task states for workflow rule evaluation
	prevStates := captureTaskStates(m.Project.Tasks)

	// Update the task's section
	task.Section = targetSection
	task.UpdatedAt = time.Now()

	// Persist with workflow rule evaluation
	return m.saveWithWorkflowRules(prevStates, task, targetColumnIndex, targetSection)
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
	// Get current column and check if we're at rightmost
	if m.ActiveColumn >= len(KanbanColumnOrder)-1 {
		// Already at rightmost column, ignore
		return nil
	}

	// Get the selected task via visible nodes
	task := m.getSelectedTask()
	if task == nil {
		return nil
	}

	// Determine target column (one to the right)
	targetColumnIndex := m.ActiveColumn + 1
	targetSection := KanbanColumnOrder[targetColumnIndex]

	// Capture previous task states for workflow rule evaluation
	prevStates := captureTaskStates(m.Project.Tasks)

	// Update the task's section
	task.Section = targetSection
	task.UpdatedAt = time.Now()

	// Persist with workflow rule evaluation
	return m.saveWithWorkflowRules(prevStates, task, targetColumnIndex, targetSection)
}

// swapTaskDown swaps the currently selected task with the task below it.
// Returns a command that persists the change with workflow rule evaluation.
// If the task is at the bottom of the column, the operation is ignored.
func (m *KanbanViewModel) swapTaskDown() tea.Cmd {
	currentSection := KanbanColumnOrder[m.ActiveColumn]
	visibleNodes := m.visibleNodesForSection(currentSection)
	cs := m.Columns[currentSection]

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
	return m.saveWithWorkflowRulesForReorder(prevStates, task, currentSection, newPosition)
}

// swapTaskUp swaps the currently selected task with the task above it.
// Returns a command that persists the change with workflow rule evaluation.
// If the task is at the top of the column, the operation is ignored.
func (m *KanbanViewModel) swapTaskUp() tea.Cmd {
	currentSection := KanbanColumnOrder[m.ActiveColumn]
	visibleNodes := m.visibleNodesForSection(currentSection)
	cs := m.Columns[currentSection]

	// Check bounds: need at least 2 visible nodes and cursor not at top
	if len(visibleNodes) < 2 || cs.Cursor <= 0 {
		return nil
	}

	task := visibleNodes[cs.Cursor].Task
	taskAbove := visibleNodes[cs.Cursor-1].Task
	if task == nil || taskAbove == nil {
		return nil
	}

	// Capture previous task states for workflow rule evaluation
	prevStates := captureTaskStates(m.Project.Tasks)

	// Swap the tasks using project's SwapTasks method
	m.Project.SwapTasks(task, taskAbove)

	// New cursor position follows the task up
	newPosition := cs.Cursor - 1

	// Persist with workflow rule evaluation
	return m.saveWithWorkflowRulesForReorder(prevStates, task, currentSection, newPosition)
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

// KanbanTaskDeletedMsg signals that a task was deleted from the Kanban view.
type KanbanTaskDeletedMsg struct {
	TaskID  string
	Section string
}

// executeDelete deletes the currently targeted task.
func (m *KanbanViewModel) executeDelete() tea.Cmd {
	if m.deleteTarget == nil {
		return nil
	}

	task := m.deleteTarget
	section := task.Section
	m.deleteTarget = nil

	// Capture previous task states for workflow rule evaluation
	prevStates := captureTaskStates(m.Project.Tasks)

	// Remove the task from the project
	for i, t := range m.Project.Tasks {
		if t.ID == task.ID {
			m.Project.Tasks = append(m.Project.Tasks[:i], m.Project.Tasks[i+1:]...)
			break
		}
	}

	// Rebuild the task tree
	m.Project.BuildTaskTree()

	// Save with workflow rules and return message
	return m.saveWithWorkflow(prevStates, func() tea.Msg {
		return KanbanTaskDeletedMsg{
			TaskID:  task.ID,
			Section: section,
		}
	})
}

// performUndo restores the project from backup.
func (m *KanbanViewModel) performUndo() tea.Cmd {
	return func() tea.Msg {
		if !storage.BackupExists(m.Project) {
			return ErrorMsg{Err: fmt.Errorf("no backup available to undo")}
		}

		if err := storage.RestoreFromBackup(m.Project); err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to undo: %w", err)}
		}

		if err := storage.LoadProject(m.Project); err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to reload after undo: %w", err)}
		}

		m.loadTasks()
		return UndoPerformedMsg{Action: "Undo"}
	}
}

// sendTaskToSection moves the selected task to the specified section.
func (m *KanbanViewModel) sendTaskToSection(targetSection string) tea.Cmd {
	task := m.getSelectedTask()
	if task == nil {
		return nil
	}

	// Find target column index
	targetColumnIndex := 0
	for i, section := range KanbanColumnOrder {
		if section == targetSection {
			targetColumnIndex = i
			break
		}
	}

	prevStates := captureTaskStates(m.Project.Tasks)
	task.Section = targetSection
	task.UpdatedAt = time.Now()

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

	// Walk up from the node above to find a valid parent
	candidate := visibleNodes[cs.Cursor-1].Task
	for candidate != nil {
		if ok, _ := task.CanIndent(candidate); ok {
			prevStates := captureTaskStates(m.Project.Tasks)
			task.ParentID = &candidate.ID
			task.UpdatedAt = time.Now()
			m.Project.BuildTaskTree()

			currentSection := KanbanColumnOrder[m.ActiveColumn]
			return m.saveWithWorkflowRulesForReorder(prevStates, task, currentSection, cs.Cursor)
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

	currentSection := KanbanColumnOrder[m.ActiveColumn]
	cs := m.CurrentColumnState()
	return m.saveWithWorkflowRulesForReorder(prevStates, task, currentSection, cs.Cursor)
}

// executeArchiveToggle archives or unarchives the project.
func (m *KanbanViewModel) executeArchiveToggle() tea.Cmd {
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
