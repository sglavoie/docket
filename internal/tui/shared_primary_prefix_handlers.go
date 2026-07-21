package tui

import (
	"fmt"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
)

// handlePriorityPrefix handles the ! prefix key for setting priority.
// Returns (cmd, handled). When handled is false the key was not
// recognized by PriorityForPrefixKey.
func handlePriorityPrefix(key string, v prefixableView) (tea.Cmd, bool) {
	priority, ok := PriorityForPrefixKey(key)
	if !ok {
		return nil, false
	}

	project := v.getProject()

	if tasks := v.getMultiSelectedTasks(); tasks != nil {
		prevStates := captureTaskStates(project.Tasks)
		count := applyBulkPriority(project, tasks, priority)
		v.clearMultiSelect()
		msg := fmt.Sprintf("Set priority %d on %d tasks (u to undo)", priority, count)
		v.setUndoContext("Set Priority", msg)
		v.setUndoTaskIDs(taskIDsFromSlice(tasks))
		v.setStatusMessage(msg)
		return v.saveAndRefreshBulk(prevStates), true
	}

	task := v.getCursorTask()
	if task == nil {
		return nil, true
	}
	if task.Priority == priority {
		return nil, true
	}
	cmd := v.singleTaskCoordinator().SetPriority(task.ID, priority)
	msg := cmd()
	if em, ok := msg.(TransactionErrorMsg); ok {
		return func() tea.Msg { return ErrorMsg{Err: em.Err} }, true
	}
	v.setMutationFlash(priorityMutationFlashBg(priority))
	return tea.Batch(func() tea.Msg { return msg }, mutationFlashTimeout(), recordRepeatPriority(priority)), true
}

// handleDueDatePrefix handles the d prefix key for setting due dates.
// Returns (cmd, handled). When handled is false the key was not recognized.
func handleDueDatePrefix(key string, v prefixableView) (tea.Cmd, bool) {
	project := v.getProject()

	if days, isClear, ok := DueDateDaysForPrefixKey(key); ok {
		if tasks := v.getMultiSelectedTasks(); tasks != nil {
			dueDate, desc := dueDateFromDays(days, isClear)
			prevStates := captureTaskStates(project.Tasks)
			count := applyBulkDueDate(project, tasks, dueDate)
			v.clearMultiSelect()
			v.setUndoContext("Set Due Date", fmt.Sprintf("%s on %d tasks", desc, count))
			v.setUndoTaskIDs(taskIDsFromSlice(tasks))
			return v.saveAndRefreshBulk(prevStates), true
		}
		task := v.getCursorTask()
		if task == nil {
			return nil, true
		}
		var cmd tea.Cmd
		var recordCmd tea.Cmd
		if isClear {
			if task.DueDate == nil {
				v.setStatusMessage("Task has no due date")
				return nil, true
			}
			cmd = v.singleTaskCoordinator().SetDueDate(task.ID, nil, false)
			recordCmd = recordRepeatClearDueDate()
		} else {
			due, _ := dueDateFromDays(days, false)
			cmd = v.singleTaskCoordinator().SetDueDate(task.ID, due, false)
			recordCmd = recordRepeatQuickDueDate(days)
		}
		msg := cmd()
		if em, ok := msg.(TransactionErrorMsg); ok {
			return func() tea.Msg { return ErrorMsg{Err: em.Err} }, true
		}
		return tea.Batch(func() tea.Msg { return msg }, recordCmd), true
	}

	if key == "d" {
		task := v.getCursorTask()
		if task == nil {
			return nil, true
		}
		return func() tea.Msg {
			return SetDueDateMsg{Task: task, Project: project}
		}, true
	}

	return nil, false
}

// handleSnoozePrefix handles the s prefix key for snoozing tasks.
// Returns (cmd, handled). When handled is false the key was not recognized.
func handleSnoozePrefix(key string, v prefixableView) (tea.Cmd, bool) {
	project := v.getProject()

	if key == "d" {
		task := v.getCursorTask()
		if task == nil {
			return nil, true
		}
		return func() tea.Msg {
			return SetSnoozeDateMsg{Task: task, Project: project}
		}, true
	}

	snoozeDate, desc, ok := resolveQuickSnoozeDate(key)
	if !ok {
		return nil, false
	}

	prevStates := captureTaskStates(project.Tasks)

	if tasks := v.getMultiSelectedTasks(); tasks != nil {
		count := 0
		for _, task := range tasks {
			if project.MutateTask(task.ID, func(t *models.Task) {
				if snoozeDate != nil {
					d := *snoozeDate // copy so each task owns its own *time.Time
					t.SnoozedUntil = &d
				} else {
					t.SnoozedUntil = nil
				}
			}) {
				count++
			}
		}
		v.clearMultiSelect()
		v.setUndoContext("Snooze", fmt.Sprintf("%s on %d tasks", desc, count))
		v.setUndoTaskIDs(taskIDsFromSlice(tasks))
		return v.saveAndRefreshBulk(prevStates), true
	}

	task := v.getCursorTask()
	if task == nil {
		return nil, true
	}
	project.MutateTask(task.ID, func(t *models.Task) {
		t.SnoozedUntil = snoozeDate
	})
	v.setUndoContext("Snooze", fmt.Sprintf("%s: %s", desc, task.Title))
	v.setUndoTaskIDs([]string{task.ID})
	return v.saveAndRefreshBulk(prevStates), true
}

// handleSkipRecurrence handles the ,k key for skipping the next occurrence of a
// recurring task. Advances the due date to the next occurrence, increments the
// skip counter, and resets the streak. Returns (cmd, handled).
func handleSkipRecurrence(v prefixableView) (tea.Cmd, bool) {
	project := v.getProject()
	task := v.getCursorTask()
	if task == nil {
		return nil, true
	}
	if !task.IsRecurring() {
		v.setStatusMessage("Task is not recurring")
		return nil, true
	}

	prevStates := captureTaskStates(project.Tasks)
	project.MutateTask(task.ID, func(t *models.Task) {
		t.SkipRecurrence()
	})
	dueStr := "none"
	if task.DueDate != nil {
		dueStr = task.DueDate.Format("Mon Jan 2, 2006")
		dueStr = AppendTimeIfHasTime(task, dueStr, "15:04")
	}
	msg := fmt.Sprintf("Skipped: next due %s (u to undo)", dueStr)
	v.setUndoContext("Skip Recurrence", task.Title)
	v.setUndoTaskIDs([]string{task.ID})
	v.setStatusMessage(msg)
	return v.saveAndRefreshBulk(prevStates), true
}

// resolveDeferDays maps a defer sub-key to a number of days and description.
// Returns (0, "", false) for unrecognized keys.
func resolveDeferDays(key string) (int, string, bool) {
	switch key {
	case "t":
		return 1, "1 day", true
	case "3":
		return 3, "3 days", true
	case "w":
		return 7, "1 week", true
	case "W":
		return 14, "2 weeks", true
	case "m":
		return 30, "1 month", true
	default:
		return 0, "", false
	}
}

// handleDeferRecurrence handles the ,D prefix for deferring a recurring task.
// Sub-keys: t=1d, 3=3d, w=1w, W=2w, m=1m. Returns (cmd, handled).
func handleDeferRecurrence(key string, v prefixableView) (tea.Cmd, bool) {
	days, desc, ok := resolveDeferDays(key)
	if !ok {
		return nil, false
	}

	project := v.getProject()
	task := v.getCursorTask()
	if task == nil {
		return nil, true
	}
	if !task.IsRecurring() {
		v.setStatusMessage("Task is not recurring")
		return nil, true
	}

	prevStates := captureTaskStates(project.Tasks)
	project.MutateTask(task.ID, func(t *models.Task) {
		t.DeferRecurrence(days)
	})
	dueStr := "none"
	if task.DueDate != nil {
		dueStr = task.DueDate.Format("Mon Jan 2, 2006")
		dueStr = AppendTimeIfHasTime(task, dueStr, "15:04")
	}
	msg := fmt.Sprintf("Deferred %s: now due %s (u to undo)", desc, dueStr)
	v.setUndoContext("Defer Recurrence", task.Title)
	v.setUndoTaskIDs([]string{task.ID})
	v.setStatusMessage(msg)
	return v.saveAndRefreshBulk(prevStates), true
}

// handleToggleKey handles toggling a boolean flag (active/pinned/dimmed) on one
// or more tasks. The spec defines which flag to toggle and the status labels.
// Returns (cmd, handled).
func handleToggleKey(v prefixableView, spec toggleSpec, undoAction, bulkLabel string) (tea.Cmd, bool) {
	project := v.getProject()

	if tasks := v.getMultiSelectedTasks(); tasks != nil {
		prevStates := captureTaskStates(project.Tasks)
		count := 0
		for _, task := range tasks {
			if project.MutateTask(task.ID, spec.mutateFn) {
				count++
			}
		}
		v.clearMultiSelect()
		msg := fmt.Sprintf("Toggled %s on %d tasks (u to undo)", bulkLabel, count)
		v.setUndoContext(undoAction, msg)
		v.setUndoTaskIDs(taskIDsFromSlice(tasks))
		v.setStatusMessage(msg)
		return v.saveAndRefreshBulk(prevStates), true
	}

	task := v.getCursorTask()
	if task == nil {
		return nil, true
	}
	var cmd tea.Cmd
	switch spec.kind {
	case toggleFlagPinned:
		cmd = v.singleTaskCoordinator().SetPinned(task.ID, !task.Pinned)
	case toggleFlagDimmed:
		cmd = v.singleTaskCoordinator().ToggleDimmed(task.ID)
	default:
		return nil, true
	}
	msg := cmd()
	if em, ok := msg.(TransactionErrorMsg); ok {
		return func() tea.Msg { return ErrorMsg{Err: em.Err} }, true
	}
	return func() tea.Msg { return msg }, true
}

// handleTogglePinnedKey handles the tp prefix key for toggling the Pinned flag.
func handleTogglePinnedKey(v prefixableView) (tea.Cmd, bool) {
	return handleToggleKey(v, togglePinned, "Toggle Pin", "pin")
}

// handleToggleDimmedKey handles the td prefix key for toggling the Dimmed flag.
func handleToggleDimmedKey(v prefixableView) (tea.Cmd, bool) {
	return handleToggleKey(v, toggleDimmed, "Toggle Dim", "dim")
}

// handleTagKey handles the T key for tag filtering or bulk tag assignment.
// selectedTags is the current tag filter selection from the view's filter state.
// Returns (cmd, handled).
func handleTagKey(v prefixableView, source PresentationID, includeCompleted bool, selectedTags map[string]bool) (tea.Cmd, bool) {
	project := v.getProject()

	if tasks := v.getMultiSelectedTasks(); tasks != nil {
		return func() tea.Msg {
			return ShowBulkTagSelectorMsg{TaskRefs: taskProjectRefsFromTasks(tasks, project)}
		}, true
	}

	return buildTagCmdFromProject(project, selectedTags, source, "Filter by Tags", includeCompleted), true
}

// handleDeleteConfirmKey handles the tD prefix key for showing delete
// confirmation dialogs. Returns true if the key was handled.
func handleDeleteConfirmKey(v deleteConfirmView) bool {
	if tasks := v.getMultiSelectedTasks(); tasks != nil {
		v.showBulkDeleteConfirm()
		return true
	}
	task := v.getCursorTask()
	if task != nil {
		v.showDeleteConfirm(task)
	}
	return true
}

// handleSharedTaskPrefix handles the t-prefix key sequence shared across
// primary views: message-only handlers (to, tc, ta, tl, te, tm, td, tP), action
// handlers (tn, tN, ts, tt), rename (tr), toggle pinned (tp), toggle dimmed (th), and delete (tD).
// The onRename callback handles view-specific rename entry. focusedTask is the
// zoom/focus target (may be nil).
func handleSharedTaskPrefix(
	key string,
	v prefixableView,
	focusedTask *models.Task,
	onRename func(task *models.Task) tea.Cmd,
) (tea.Cmd, bool) {
	task := v.getCursorTask()
	project := v.getProject()

	// Multi-select getter for shared handlers
	getMultiTasks := func() []*models.Task {
		return v.getMultiSelectedTasks()
	}

	// Try shared message-only handlers first (to, tc, ta, tl, te, tm)
	if task != nil {
		if handled, cmd := handleTaskPrefixMsgKey(key, task, project); handled {
			return cmd, true
		}
	}

	// Try shared action handlers (tn, tN, ts, tt)
	if handled, cmd := handleTaskActionPrefixKey(key, task, project, focusedTask, getMultiTasks); handled {
		return cmd, true
	}

	switch key {
	case "r":
		if task != nil && onRename != nil {
			return onRename(task), true
		}
		return nil, true
	case "p":
		cmd, _ := handleTogglePinnedKey(v)
		return cmd, true
	case "h":
		cmd, _ := handleToggleDimmedKey(v)
		return cmd, true
	case "D":
		if dv, ok := v.(deleteConfirmView); ok {
			handleDeleteConfirmKey(dv)
		}
		return nil, true
	}
	return nil, false
}
