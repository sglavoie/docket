package tui

import (
	tea "charm.land/bubbletea/v2"
)

// crossHandlePriorityMutation resolves the owning project from the cursor row and
// applies a priority change through the single-task coordinator (app layer). The
// coordinator commits one ledger-backed transaction; the shell owns reload and
// the status line via handleTransactionResult.
// It is called from handlePrefixKey when prefix="!" and m.project==nil.
//
// Returns (cmd, true) when key maps to a priority level and was consumed.
// Returns (nil, false) when no priority mapping exists for the key.
func (m *OutlinerModel) crossHandlePriorityMutation(key string) (tea.Cmd, bool) {
	priority, ok := PriorityForPrefixKey(key)
	if !ok {
		return nil, false
	}
	row, project, valid := m.crossGetMutationTarget()
	if !valid {
		return nil, true
	}
	return m.taskItem.SetPriority(taskIdentityFor(row.task, project), priority), true
}

// crossHandleSnoozeMutation resolves the owning project from the cursor row and
// applies a snooze date mutation through the single-task coordinator.
// It is called from handlePrefixKey when prefix="s" and m.project==nil.
//
// "sd" emits SetSnoozeDateMsg (date picker). All other recognized snooze keys
// commit through the coordinator, which reloads the touched project.
//
// Returns (cmd, true) when the key was consumed.
// Returns (nil, false) when the key is not a recognized snooze sub-key.
func (m *OutlinerModel) crossHandleSnoozeMutation(key string) (tea.Cmd, bool) {
	if key == "d" {
		row, proj, valid := m.crossGetMutationTarget()
		if !valid {
			return nil, true
		}
		task := row.task
		return func() tea.Msg {
			return SetSnoozeDateMsg{Task: task, Project: proj}
		}, true
	}

	snoozeDate, _, ok := resolveQuickSnoozeDate(key)
	if !ok {
		return nil, false
	}
	row, project, valid := m.crossGetMutationTarget()
	if !valid {
		return nil, true
	}
	return m.taskItem.Snooze(taskIdentityFor(row.task, project), snoozeDate), true
}

// crossHandleDueDateMutation handles the d prefix in cross-project mode.
// "dd" emits SetDueDateMsg (date picker). Quick keys (dt/dm/dw/dn/d0) commit
// through the single-task coordinator.
// Returns (cmd, true) when the key was consumed.
func (m *OutlinerModel) crossHandleDueDateMutation(key string) (tea.Cmd, bool) {
	if key == "d" {
		row, proj, valid := m.crossGetMutationTarget()
		if !valid {
			return nil, true
		}
		task := row.task
		return func() tea.Msg {
			return SetDueDateMsg{Task: task, Project: proj}
		}, true
	}

	days, isClear, ok := DueDateDaysForPrefixKey(key)
	if !ok {
		return nil, false
	}
	row, project, valid := m.crossGetMutationTarget()
	if !valid {
		return nil, true
	}
	dueDate, _ := dueDateFromDays(days, isClear)
	return m.taskItem.SetDueDate(taskIdentityFor(row.task, project), dueDate, false), true
}

// crossHandleSkipRecurrence handles the ,k key in cross-project mode.
// Advances the due date to the next occurrence of a recurring task through the
// single-task coordinator (app SkipRecurrence), mirroring handleSkipRecurrence
// for single-project views.
//
// Returns (cmd, true) when the key was consumed.
func (m *OutlinerModel) crossHandleSkipRecurrence() (tea.Cmd, bool) {
	row, project, valid := m.crossGetMutationTarget()
	if !valid {
		return nil, true
	}
	if !row.task.IsRecurring() {
		m.setStatus("Task is not recurring")
		return nil, true
	}
	return m.taskItem.SkipRecurrence(taskIdentityFor(row.task, project)), true
}

// crossHandleDeferRecurrence handles the ,D prefix in cross-project mode.
// Sub-keys: t=1d, 3=3d, w=1w, W=2w, m=1m. Defers through the single-task
// coordinator (app DeferRecurrence). Returns (nil, false) for unknown keys.
func (m *OutlinerModel) crossHandleDeferRecurrence(key string) (tea.Cmd, bool) {
	days, _, ok := resolveDeferDays(key)
	if !ok {
		return nil, false
	}
	row, project, valid := m.crossGetMutationTarget()
	if !valid {
		return nil, true
	}
	if !row.task.IsRecurring() {
		m.setStatus("Task is not recurring")
		return nil, true
	}
	return m.taskItem.DeferRecurrence(taskIdentityFor(row.task, project), days), true
}
