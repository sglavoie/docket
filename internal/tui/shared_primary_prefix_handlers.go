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
		msg := fmt.Sprintf("Set priority %d on %d tasks", priority, count)
		v.setUndoContext("Set Priority", msg)
		v.setStatusMessage(msg)
		return v.saveAndRefreshBulk(prevStates), true
	}

	task := v.getCursorTask()
	if task == nil {
		return nil, true
	}
	v.setUndoContext("Set Priority", fmt.Sprintf("P%d: %s", priority, task.Title))
	result := applyPriorityChange(v.workflowPersistFn(), project, task.ID, priority)
	if result.Err != nil {
		return func() tea.Msg { return ErrorMsg{Err: result.Err} }, true
	}
	if result.StatusMessage != "" {
		v.setStatusMessage(result.StatusMessage)
	}
	v.refreshView()
	return result.Cmd, true
}

// handleStagePrefix handles the . prefix key for setting workflow stage.
// Returns (cmd, handled). When handled is false the key was not recognized.
func handleStagePrefix(key string, v prefixableView, stages []string) (tea.Cmd, bool) {
	stage := StageForPrefixKey(key, stages)
	if stage == nil {
		return nil, false
	}

	project := v.getProject()

	if tasks := v.getMultiSelectedTasks(); tasks != nil {
		prevStates := captureTaskStates(project.Tasks)
		count := 0
		for _, t := range tasks {
			if t.Stage != *stage {
				project.MutateTask(t.ID, func(mt *models.Task) {
					mt.Stage = *stage
				})
				count++
			}
		}
		v.clearMultiSelect()
		label := *stage
		if label == "" {
			label = "cleared"
		}
		msg := fmt.Sprintf("Set stage %s on %d tasks", label, count)
		v.setUndoContext("Set Stage", msg)
		v.setStatusMessage(msg)
		return v.saveAndRefreshBulk(prevStates), true
	}

	task := v.getCursorTask()
	if task == nil {
		return nil, true
	}
	label := *stage
	if label == "" {
		label = "(cleared)"
	}
	v.setUndoContext("Set Stage", fmt.Sprintf("%s: %s", label, task.Title))
	result := applyStageChange(v.workflowPersistFn(), project, task.ID, *stage)
	if result.Err != nil {
		return func() tea.Msg { return ErrorMsg{Err: result.Err} }, true
	}
	if result.StatusMessage != "" {
		v.setStatusMessage(result.StatusMessage)
	}
	v.refreshView()
	return result.Cmd, true
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
			return v.saveAndRefreshBulk(prevStates), true
		}
		task := v.getCursorTask()
		if task == nil {
			return nil, true
		}
		var result TaskOpResult
		if isClear {
			v.setUndoContext("Set Due Date", fmt.Sprintf("cleared: %s", task.Title))
			result = applyClearDueDate(v.workflowPersistFn(), project, task.ID)
		} else {
			v.setUndoContext("Set Due Date", fmt.Sprintf("%s: %s", dueDateLabel(days), task.Title))
			result = applyQuickDueDate(v.workflowPersistFn(), project, task.ID, days)
		}
		if result.Err != nil {
			return func() tea.Msg { return ErrorMsg{Err: result.Err} }, true
		}
		if result.StatusMessage != "" {
			v.setStatusMessage(result.StatusMessage)
		}
		v.refreshView()
		return result.Cmd, true
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
				t.SnoozedUntil = snoozeDate
			}) {
				count++
			}
		}
		v.clearMultiSelect()
		v.setUndoContext("Snooze", fmt.Sprintf("%s on %d tasks", desc, count))
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
	}
	msg := fmt.Sprintf("Skipped: next due %s", dueStr)
	v.setUndoContext("Skip Recurrence", task.Title)
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
	}
	msg := fmt.Sprintf("Deferred %s: now due %s", desc, dueStr)
	v.setUndoContext("Defer Recurrence", task.Title)
	v.setStatusMessage(msg)
	return v.saveAndRefreshBulk(prevStates), true
}

// handleToggleActiveKey handles the * key for toggling the Active flag.
// Returns (cmd, handled).
func handleToggleActiveKey(v prefixableView) (tea.Cmd, bool) {
	project := v.getProject()

	if tasks := v.getMultiSelectedTasks(); tasks != nil {
		prevStates := captureTaskStates(project.Tasks)
		count := 0
		for _, task := range tasks {
			task.ToggleActive()
			count++
		}
		v.clearMultiSelect()
		msg := fmt.Sprintf("Toggled active on %d tasks", count)
		v.setUndoContext("Toggle Active", msg)
		v.setStatusMessage(msg)
		return v.saveAndRefreshBulk(prevStates), true
	}

	task := v.getCursorTask()
	if task == nil {
		return nil, true
	}
	v.setUndoContext("Toggle Active", task.Title)
	result := applyToggleActive(v.workflowPersistFn(), project, task.ID)
	if result.Err != nil {
		return func() tea.Msg { return ErrorMsg{Err: result.Err} }, true
	}
	cmds := []tea.Cmd{result.Cmd}
	if result.StatusMessage != "" {
		v.setStatusMessage(result.StatusMessage)
	}
	v.refreshView()
	return tea.Batch(cmds...), true
}

// handleTogglePinnedKey handles the tp prefix key for toggling the Pinned flag.
// Returns (cmd, handled).
func handleTogglePinnedKey(v prefixableView) (tea.Cmd, bool) {
	project := v.getProject()

	if tasks := v.getMultiSelectedTasks(); tasks != nil {
		prevStates := captureTaskStates(project.Tasks)
		count := 0
		for _, task := range tasks {
			task.TogglePinned()
			count++
		}
		v.clearMultiSelect()
		msg := fmt.Sprintf("Toggled pin on %d tasks", count)
		v.setUndoContext("Toggle Pin", msg)
		v.setStatusMessage(msg)
		return v.saveAndRefreshBulk(prevStates), true
	}

	task := v.getCursorTask()
	if task == nil {
		return nil, true
	}
	v.setUndoContext("Toggle Pin", task.Title)
	result := applyTogglePinned(v.workflowPersistFn(), project, task.ID)
	if result.Err != nil {
		return func() tea.Msg { return ErrorMsg{Err: result.Err} }, true
	}
	cmds := []tea.Cmd{result.Cmd}
	if result.StatusMessage != "" {
		v.setStatusMessage(result.StatusMessage)
	}
	v.refreshView()
	return tea.Batch(cmds...), true
}

// handleToggleDimmedKey handles the td prefix key for toggling the Dimmed flag.
// Returns (cmd, handled).
func handleToggleDimmedKey(v prefixableView) (tea.Cmd, bool) {
	project := v.getProject()

	if tasks := v.getMultiSelectedTasks(); tasks != nil {
		prevStates := captureTaskStates(project.Tasks)
		count := 0
		for _, task := range tasks {
			task.ToggleDimmed()
			count++
		}
		v.clearMultiSelect()
		msg := fmt.Sprintf("Toggled dim on %d tasks", count)
		v.setUndoContext("Toggle Dim", msg)
		v.setStatusMessage(msg)
		return v.saveAndRefreshBulk(prevStates), true
	}

	task := v.getCursorTask()
	if task == nil {
		return nil, true
	}
	v.setUndoContext("Toggle Dim", task.Title)
	result := applyToggleDimmed(v.workflowPersistFn(), project, task.ID)
	if result.Err != nil {
		return func() tea.Msg { return ErrorMsg{Err: result.Err} }, true
	}
	cmds := []tea.Cmd{result.Cmd}
	if result.StatusMessage != "" {
		v.setStatusMessage(result.StatusMessage)
	}
	v.refreshView()
	return tea.Batch(cmds...), true
}

// handleTagKey handles the T key for tag filtering or bulk tag assignment.
// selectedTags is the current tag filter selection from the view's filter state.
// Returns (cmd, handled).
func handleTagKey(v prefixableView, source ViewType, includeCompleted bool, selectedTags map[string]bool) (tea.Cmd, bool) {
	project := v.getProject()

	if tasks := v.getMultiSelectedTasks(); tasks != nil {
		return func() tea.Msg {
			return ShowBulkTagSelectorMsg{Tasks: tasks, Project: project}
		}, true
	}

	tags := BuildTagList(project.Tasks, includeCompleted)
	return func() tea.Msg {
		return ShowTagFilterMsg{Tags: tags, Selected: selectedTags, Source: source, Title: "Filter by Tags"}
	}, true
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
// primary views: message-only handlers (to, tc, ta, tl, te, tm), action
// handlers (tn, tN, ts, tt), rename (tr), toggle pinned (tp), toggle dimmed (td), and delete (tD).
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
	case "d":
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
