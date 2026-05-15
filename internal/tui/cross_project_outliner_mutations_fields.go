package tui

import (
	"fmt"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
)

// crossHandlePriorityMutation resolves the owning project from the cursor row,
// applies a priority change, and forces a row rebuild via rebuildRows.
// It is called from handlePrefixKey when prefix="!" and m.project==nil.
//
// Returns (cmd, true) when key maps to a priority level and was consumed.
// Returns (nil, false) when no priority mapping exists for the key.
func (m *OutlinerModel) crossHandlePriorityMutation(key string) (tea.Cmd, bool) {
	priority, ok := PriorityForPrefixKey(key)
	if !ok {
		return nil, false
	}
	row, proj, valid := m.crossGetMutationTarget()
	if !valid {
		return nil, true
	}
	m.crossUndoCtx.Set("Set Priority", row.task.Title)
	m.crossUndoCtx.ProjectIDs = []string{proj.ID}
	pf := m.crossPersistFn(proj)
	result := applyPriorityChange(pf, proj, row.task.ID, priority)
	if result.Err != nil {
		return func() tea.Msg { return ErrorMsg{Err: result.Err} }, true
	}
	if result.StatusMessage != "" {
		m.setStatus(result.StatusMessage)
	}
	m.mutationFlash = true
	m.mutationFlashBg = priorityMutationFlashBg(priority)
	m.rebuildRows()
	return tea.Batch(result.Cmd, mutationFlashTimeout()), true
}

// crossHandleSnoozeMutation resolves the owning project from the cursor row,
// applies a snooze date mutation, and forces a row rebuild via rebuildRows.
// It is called from handlePrefixKey when prefix="s" and m.project==nil.
//
// "sd" emits SetSnoozeDateMsg (date picker). All other recognized snooze keys
// mutate in-place, call Backup() then Save(), and rebuild rows synchronously.
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

	snoozeDate, desc, ok := resolveQuickSnoozeDate(key)
	if !ok {
		return nil, false
	}
	row, proj, valid := m.crossGetMutationTarget()
	if !valid {
		return nil, true
	}

	m.crossUndoCtx.Set("Snooze", row.task.Title)
	m.crossUndoCtx.ProjectIDs = []string{proj.ID}
	if m.config != nil && m.config.UndoHistoryDepth != nil && *m.config.UndoHistoryDepth > 0 {
		_ = m.projectService.RecordUndoSnapshot(proj, "Snooze", row.task.Title, *m.config.UndoHistoryDepth, nil)
	}
	if err := m.projectService.Backup(proj); err != nil {
		return func() tea.Msg { return ErrorMsg{Err: fmt.Errorf("backup failed: %w", err)} }, true
	}
	proj.MutateTask(row.task.ID, func(t *models.Task) {
		t.SnoozedUntil = snoozeDate
	})
	if err := m.projectService.Save(proj); err != nil {
		return func() tea.Msg { return ErrorMsg{Err: fmt.Errorf("save failed: %w", err)} }, true
	}
	_ = m.projectService.Load(proj)

	if snoozeDate == nil {
		m.setStatus("Snooze cleared")
	} else {
		m.setStatus(fmt.Sprintf("Snoozed until %s (%s)", snoozeDate.Format("Mon Jan 2"), desc))
	}
	m.rebuildRows()
	return nil, true
}

// crossHandleDueDateMutation handles the d prefix in cross-project mode.
// "dd" emits SetDueDateMsg (date picker). Quick keys (dt/dm/dw/dn/d0) mutate inline.
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
	row, proj, valid := m.crossGetMutationTarget()
	if !valid {
		return nil, true
	}
	pf := m.crossPersistFn(proj)
	var result TaskOpResult
	if isClear {
		m.crossUndoCtx.Set("Set Due Date", fmt.Sprintf("cleared: %s", row.task.Title))
		result = applyClearDueDate(pf, proj, row.task.ID)
	} else {
		m.crossUndoCtx.Set("Set Due Date", fmt.Sprintf("%s: %s", dueDateLabel(days), row.task.Title))
		result = applyQuickDueDate(pf, proj, row.task.ID, days)
	}
	m.crossUndoCtx.ProjectIDs = []string{proj.ID}
	if result.Err != nil {
		return func() tea.Msg { return ErrorMsg{Err: result.Err} }, true
	}
	if result.StatusMessage != "" {
		m.setStatus(result.StatusMessage)
	}
	m.rebuildRows()
	return result.Cmd, true
}

// crossHandleSkipRecurrence handles the ,k key in cross-project mode.
// Advances the due date to the next occurrence of a recurring task, mirroring
// handleSkipRecurrence for single-project views.
//
// Returns (cmd, true) when the key was consumed.
func (m *OutlinerModel) crossHandleSkipRecurrence() (tea.Cmd, bool) {
	row, proj, valid := m.crossGetMutationTarget()
	if !valid {
		return nil, true
	}
	if !row.task.IsRecurring() {
		m.setStatus("Task is not recurring")
		return nil, true
	}

	m.crossUndoCtx.Set("Skip Recurrence", row.task.Title)
	m.crossUndoCtx.ProjectIDs = []string{proj.ID}
	if m.config != nil && m.config.UndoHistoryDepth != nil && *m.config.UndoHistoryDepth > 0 {
		_ = m.projectService.RecordUndoSnapshot(proj, "Skip Recurrence", row.task.Title, *m.config.UndoHistoryDepth, nil)
	}
	if err := m.projectService.Backup(proj); err != nil {
		return func() tea.Msg { return ErrorMsg{Err: fmt.Errorf("backup failed: %w", err)} }, true
	}
	proj.MutateTask(row.task.ID, func(t *models.Task) {
		t.SkipRecurrence()
	})
	if err := m.projectService.Save(proj); err != nil {
		return func() tea.Msg { return ErrorMsg{Err: fmt.Errorf("save failed: %w", err)} }, true
	}
	_ = m.projectService.Load(proj)

	dueStr := "none"
	if row.task.DueDate != nil {
		dueStr = row.task.DueDate.Format("Mon Jan 2, 2006")
		dueStr = AppendTimeIfHasTime(row.task, dueStr, "15:04")
	}
	m.setStatus(fmt.Sprintf("Skipped: next due %s (u to undo)", dueStr))
	m.rebuildRows()
	return nil, true
}

// crossHandleDeferRecurrence handles the ,D prefix in cross-project mode.
// Sub-keys: t=1d, 3=3d, w=1w, W=2w, m=1m. Returns (nil, false) for unknown keys.
func (m *OutlinerModel) crossHandleDeferRecurrence(key string) (tea.Cmd, bool) {
	days, desc, ok := resolveDeferDays(key)
	if !ok {
		return nil, false
	}
	row, proj, valid := m.crossGetMutationTarget()
	if !valid {
		return nil, true
	}
	if !row.task.IsRecurring() {
		m.setStatus("Task is not recurring")
		return nil, true
	}

	m.crossUndoCtx.Set("Defer Recurrence", row.task.Title)
	m.crossUndoCtx.ProjectIDs = []string{proj.ID}
	if m.config != nil && m.config.UndoHistoryDepth != nil && *m.config.UndoHistoryDepth > 0 {
		_ = m.projectService.RecordUndoSnapshot(proj, "Defer Recurrence", row.task.Title, *m.config.UndoHistoryDepth, nil)
	}
	if err := m.projectService.Backup(proj); err != nil {
		return func() tea.Msg { return ErrorMsg{Err: fmt.Errorf("backup failed: %w", err)} }, true
	}
	proj.MutateTask(row.task.ID, func(t *models.Task) {
		t.DeferRecurrence(days)
	})
	if err := m.projectService.Save(proj); err != nil {
		return func() tea.Msg { return ErrorMsg{Err: fmt.Errorf("save failed: %w", err)} }, true
	}
	_ = m.projectService.Load(proj)

	dueStr := "none"
	if row.task.DueDate != nil {
		dueStr = row.task.DueDate.Format("Mon Jan 2, 2006")
		dueStr = AppendTimeIfHasTime(row.task, dueStr, "15:04")
	}
	m.setStatus(fmt.Sprintf("Deferred %s: now due %s (u to undo)", desc, dueStr))
	m.rebuildRows()
	return nil, true
}

// crossHandleStageMutation handles the . prefix in cross-project mode.
// Maps key to a stage name and applies the change.
// Returns (cmd, true) when the key was consumed.
func (m *OutlinerModel) crossHandleStageMutation(key string) (tea.Cmd, bool) {
	stage := StageForPrefixKey(key, m.config.Stages)
	if stage == nil {
		return nil, false
	}
	row, proj, valid := m.crossGetMutationTarget()
	if !valid {
		return nil, true
	}
	label := *stage
	if label == "" {
		label = "(cleared)"
	}
	m.crossUndoCtx.Set("Set Stage", fmt.Sprintf("%s: %s", label, row.task.Title))
	m.crossUndoCtx.ProjectIDs = []string{proj.ID}
	pf := m.crossPersistFn(proj)
	result := applyStageChange(pf, proj, row.task.ID, *stage)
	if result.Err != nil {
		return func() tea.Msg { return ErrorMsg{Err: result.Err} }, true
	}
	if result.StatusMessage != "" {
		m.setStatus(result.StatusMessage)
	}
	m.rebuildRows()
	return result.Cmd, true
}
