package tui

import (
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/workflow"
)

// moveTaskLeft moves the currently selected task to the column on the left.
// Returns a command that persists the change with workflow rule evaluation.
// If the task is already in the leftmost column, the operation is ignored.
func (m *KanbanViewModel) moveTaskLeft() tea.Cmd {
	// Get current column and check if we're at leftmost
	if m.ActiveColumn <= 0 {
		// Already at leftmost column, ignore
		return nil
	}

	// Get the selected task in the current column
	currentSection := KanbanColumnOrder[m.ActiveColumn]
	tasks := m.Tasks[currentSection]
	cs := m.Columns[currentSection]

	if len(tasks) == 0 || !cs.InBounds(len(tasks)) {
		// No task selected or cursor out of bounds
		return nil
	}

	task := tasks[cs.Cursor]
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
	// Capture references for closure
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

		return KanbanTaskMovedMsg{
			Task:              task,
			TargetColumnIndex: targetColumnIndex,
			TargetSection:     targetSection,
		}
	}
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

	// Get the selected task in the current column
	currentSection := KanbanColumnOrder[m.ActiveColumn]
	tasks := m.Tasks[currentSection]
	cs := m.Columns[currentSection]

	if len(tasks) == 0 || !cs.InBounds(len(tasks)) {
		// No task selected or cursor out of bounds
		return nil
	}

	task := tasks[cs.Cursor]
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

// KanbanTaskMovedMsg signals that a task was moved to a different column.
type KanbanTaskMovedMsg struct {
	Task              *models.Task
	TargetColumnIndex int
	TargetSection     string
}
