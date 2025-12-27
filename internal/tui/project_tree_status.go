package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/sglavoie/tt/internal/models"
)

// statusPredicate determines if a task is eligible for a status change.
type statusPredicate func(*models.Task) bool

// statusAction applies a status change to a task.
type statusAction func(*models.Task)

// statusValidator checks if a task can be changed; returns error message if blocked, empty string if allowed.
type statusValidator func(*models.Task) string

// handleStatusChange applies a status change to selected tasks or the current task.
// It handles multi-select mode by applying the change to all selected tasks that pass
// the predicate, then clears selection. For single-task mode, it applies the change
// to the task at the cursor if it passes the predicate.
func (m *ProjectTreeModel) handleStatusChange(
	visible []*TreeNode,
	predicate statusPredicate,
	action statusAction,
	undoAction string,
) (bool, tea.Cmd) {
	if m.multiSelectMode && len(m.selectedTasks) > 0 {
		count := 0
		for _, task := range m.getSelectedTasks() {
			if predicate(task) {
				action(task)
				count++
			}
		}
		m.clearSelection()
		if count > 0 {
			m.setUndoContext(undoAction, fmt.Sprintf("%d tasks", count))
		}
		return true, m.saveAndRefresh()
	}
	if CursorInBounds(m.cursor, len(visible)) {
		task := visible[m.cursor].Task
		if predicate(task) {
			action(task)
			m.setUndoContext(undoAction, fmt.Sprintf("#%d %s", task.ShortID, task.Title))
			return true, m.saveAndRefresh()
		}
	}
	return true, nil
}

// handleStatusChangeWithValidation applies a status change with per-task validation.
// Unlike handleStatusChange, this variant can skip individual tasks based on validation
// and reports skipped tasks via an error message. Used for operations like complete
// where dependency checking may block individual tasks.
func (m *ProjectTreeModel) handleStatusChangeWithValidation(
	visible []*TreeNode,
	predicate statusPredicate,
	validator statusValidator,
	action statusAction,
	actionVerb string,
	undoAction string,
) (bool, tea.Cmd) {
	if m.multiSelectMode && len(m.selectedTasks) > 0 {
		var skippedIDs []string
		actionCount := 0
		for _, task := range m.getSelectedTasks() {
			if !predicate(task) {
				continue
			}
			if errMsg := validator(task); errMsg != "" {
				skippedIDs = append(skippedIDs, fmt.Sprintf("#%d", task.ShortID))
				continue
			}
			action(task)
			actionCount++
		}
		m.clearSelection()
		if actionCount > 0 {
			m.setUndoContext(undoAction, fmt.Sprintf("%d tasks", actionCount))
		}
		if len(skippedIDs) > 0 {
			var msg string
			if actionCount == 0 {
				msg = fmt.Sprintf("no tasks %s: %s have incomplete dependencies", actionVerb, strings.Join(skippedIDs, ", "))
			} else {
				msg = fmt.Sprintf("%s %d, skipped %s (incomplete dependencies)", actionVerb, actionCount, strings.Join(skippedIDs, ", "))
			}
			return true, tea.Batch(m.saveAndRefresh(), func() tea.Msg {
				return ErrorMsg{Err: fmt.Errorf("%s", msg)}
			})
		}
		return true, m.saveAndRefresh()
	}
	if CursorInBounds(m.cursor, len(visible)) {
		task := visible[m.cursor].Task
		if predicate(task) {
			if errMsg := validator(task); errMsg != "" {
				return true, func() tea.Msg {
					return ErrorMsg{Err: fmt.Errorf("%s", errMsg)}
				}
			}
			action(task)
			m.setUndoContext(undoAction, fmt.Sprintf("#%d %s", task.ShortID, task.Title))
			return true, m.saveAndRefresh()
		}
	}
	return true, nil
}

// handleStatusKeys handles s, m, b, x, o - status change operations.
// Returns true if the key was handled.
func (m *ProjectTreeModel) handleStatusKeys(key string, visible []*TreeNode) (bool, tea.Cmd) {
	switch key {
	case "s":
		return m.handleStartTask(visible)
	case "m":
		return m.handleCompleteTask(visible)
	case "b":
		return m.handleBlockTask(visible)
	case "x":
		return m.handleCancelTask(visible)
	case "o":
		return m.handleReopenTask(visible)
	case "`":
		return m.handleCycleStatusForward(visible)
	case "~":
		return m.handleCycleStatusBackward(visible)
	}
	return false, nil
}

// handleStartTask starts a task (in_progress).
func (m *ProjectTreeModel) handleStartTask(visible []*TreeNode) (bool, tea.Cmd) {
	return m.handleStatusChange(visible,
		func(t *models.Task) bool {
			return t.Status == models.StatusTodo || t.Status == models.StatusBlocked
		},
		func(t *models.Task) { t.Start() },
		"Start Task",
	)
}

// handleCompleteTask marks a task as complete (done).
func (m *ProjectTreeModel) handleCompleteTask(visible []*TreeNode) (bool, tea.Cmd) {
	return m.handleStatusChangeWithValidation(visible,
		func(t *models.Task) bool { return t.Status.IsActive() },
		func(t *models.Task) string {
			incompleteDeps := m.project.GetIncompleteDependencies(t)
			if len(incompleteDeps) == 0 {
				return ""
			}
			depNames := make([]string, len(incompleteDeps))
			for i, dep := range incompleteDeps {
				depNames[i] = fmt.Sprintf("#%d", dep.ShortID)
			}
			return fmt.Sprintf("cannot complete: %d incomplete dependencies (%s)",
				len(incompleteDeps), strings.Join(depNames, ", "))
		},
		func(t *models.Task) { t.Complete() },
		"completed",
		"Complete Task",
	)
}

// handleBlockTask blocks a task.
func (m *ProjectTreeModel) handleBlockTask(visible []*TreeNode) (bool, tea.Cmd) {
	return m.handleStatusChange(visible,
		func(t *models.Task) bool {
			return t.Status.IsActive() && t.Status != models.StatusBlocked
		},
		func(t *models.Task) { t.Block() },
		"Block Task",
	)
}

// handleCancelTask cancels a task.
func (m *ProjectTreeModel) handleCancelTask(visible []*TreeNode) (bool, tea.Cmd) {
	return m.handleStatusChange(visible,
		func(t *models.Task) bool { return t.Status.IsActive() },
		func(t *models.Task) { t.Cancel() },
		"Cancel Task",
	)
}

// handleReopenTask reopens a task (back to todo).
func (m *ProjectTreeModel) handleReopenTask(visible []*TreeNode) (bool, tea.Cmd) {
	return m.handleStatusChange(visible,
		func(t *models.Task) bool { return t.Status != models.StatusTodo },
		func(t *models.Task) { t.Reopen() },
		"Reopen Task",
	)
}

// handleCycleStatusForward cycles status: todo -> in_progress -> done -> todo.
func (m *ProjectTreeModel) handleCycleStatusForward(visible []*TreeNode) (bool, tea.Cmd) {
	if m.multiSelectMode && len(m.selectedTasks) > 0 {
		var skippedIDs []string
		cycledCount := 0
		for _, task := range m.getSelectedTasks() {
			switch task.Status {
			case models.StatusTodo:
				task.Start()
				cycledCount++
			case models.StatusInProgress:
				if incompleteDeps := m.project.GetIncompleteDependencies(task); len(incompleteDeps) > 0 {
					skippedIDs = append(skippedIDs, fmt.Sprintf("#%d", task.ShortID))
					continue
				}
				task.Complete()
				cycledCount++
			case models.StatusDone:
				task.Reopen()
				cycledCount++
			}
		}
		m.clearSelection()
		if cycledCount > 0 {
			m.setUndoContext("Cycle Status", fmt.Sprintf("%d tasks", cycledCount))
		}
		if len(skippedIDs) > 0 {
			var msg string
			if cycledCount == 0 {
				msg = fmt.Sprintf("no tasks cycled: %s have incomplete dependencies", strings.Join(skippedIDs, ", "))
			} else {
				msg = fmt.Sprintf("cycled %d, skipped %s (incomplete dependencies)", cycledCount, strings.Join(skippedIDs, ", "))
			}
			return true, tea.Batch(m.saveAndRefresh(), func() tea.Msg {
				return ErrorMsg{Err: fmt.Errorf("%s", msg)}
			})
		}
		return true, m.saveAndRefresh()
	}
	if CursorInBounds(m.cursor, len(visible)) {
		task := visible[m.cursor].Task
		switch task.Status {
		case models.StatusTodo:
			task.Start()
			m.setUndoContext("Start Task", fmt.Sprintf("#%d %s", task.ShortID, task.Title))
			return true, m.saveAndRefresh()
		case models.StatusInProgress:
			if incompleteDeps := m.project.GetIncompleteDependencies(task); len(incompleteDeps) > 0 {
				depNames := make([]string, len(incompleteDeps))
				for i, dep := range incompleteDeps {
					depNames[i] = fmt.Sprintf("#%d", dep.ShortID)
				}
				return true, func() tea.Msg {
					return ErrorMsg{Err: fmt.Errorf("cannot complete: %d incomplete dependencies (%s)",
						len(incompleteDeps), strings.Join(depNames, ", "))}
				}
			}
			task.Complete()
			m.setUndoContext("Complete Task", fmt.Sprintf("#%d %s", task.ShortID, task.Title))
			return true, m.saveAndRefresh()
		case models.StatusDone:
			task.Reopen()
			m.setUndoContext("Reopen Task", fmt.Sprintf("#%d %s", task.ShortID, task.Title))
			return true, m.saveAndRefresh()
		}
	}
	return true, nil
}

// handleCycleStatusBackward cycles status backward: todo -> done -> in_progress -> todo.
func (m *ProjectTreeModel) handleCycleStatusBackward(visible []*TreeNode) (bool, tea.Cmd) {
	if m.multiSelectMode && len(m.selectedTasks) > 0 {
		count := 0
		for _, task := range m.getSelectedTasks() {
			switch task.Status {
			case models.StatusTodo:
				task.Complete()
				count++
			case models.StatusInProgress:
				task.Reopen()
				count++
			case models.StatusDone:
				task.Start()
				count++
			}
		}
		m.clearSelection()
		if count > 0 {
			m.setUndoContext("Cycle Status", fmt.Sprintf("%d tasks", count))
		}
		return true, m.saveAndRefresh()
	}
	if CursorInBounds(m.cursor, len(visible)) {
		task := visible[m.cursor].Task
		switch task.Status {
		case models.StatusTodo:
			task.Complete()
			m.setUndoContext("Complete Task", fmt.Sprintf("#%d %s", task.ShortID, task.Title))
			return true, m.saveAndRefresh()
		case models.StatusInProgress:
			task.Reopen()
			m.setUndoContext("Reopen Task", fmt.Sprintf("#%d %s", task.ShortID, task.Title))
			return true, m.saveAndRefresh()
		case models.StatusDone:
			task.Start()
			m.setUndoContext("Start Task", fmt.Sprintf("#%d %s", task.ShortID, task.Title))
			return true, m.saveAndRefresh()
		}
	}
	return true, nil
}
