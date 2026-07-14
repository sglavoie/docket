package tui

import (
	"fmt"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
)

// handleTaskDateMessages handles task date-related messages (due dates, snooze).
// Returns true if the message was handled.
func (m *AppModel) handleTaskDateMessages(msg tea.Msg) (bool, tea.Cmd) {
	switch msg := msg.(type) {
	case SetDueDateMsg:
		return m.handleSetDueDate(msg)

	case SetSnoozeDateMsg:
		return m.handleSetSnoozeDate(msg)

	case SnoozeDateChangedMsg:
		return m.handleSnoozeDateChanged(msg)

	case DueDateChangedMsg:
		return m.handleDueDateChanged(msg)
	}
	return false, nil
}

// handleSetDueDate handles the SetDueDateMsg.
func (m *AppModel) handleSetDueDate(msg SetDueDateMsg) (bool, tea.Cmd) {
	if refs := validTaskProjectRefs(msg.TaskRefs); len(refs) > 0 {
		m.pendingBulkTasks = refs
		first := refs[0]
		weekEndDay := m.runtime.Settings().GetWeekEndDay()
		m.showView(ViewDateInput, NewDateInputModel(first.Task, first.Project, weekEndDay))
		return true, nil
	}
	m.pendingBulkTasks = nil

	if msg.Task != nil && msg.Project != nil {
		weekEndDay := m.runtime.Settings().GetWeekEndDay()
		m.showView(ViewDateInput, NewDateInputModel(msg.Task, msg.Project, weekEndDay))
	}
	return true, nil
}

// handleSetSnoozeDate handles the SetSnoozeDateMsg.
func (m *AppModel) handleSetSnoozeDate(msg SetSnoozeDateMsg) (bool, tea.Cmd) {
	if refs := validTaskProjectRefs(msg.TaskRefs); len(refs) > 0 {
		m.pendingBulkTasks = refs
		first := refs[0]
		m.showView(ViewSnoozeInput, NewSnoozeInputModel(first.Task, first.Project))
		return true, nil
	}
	m.pendingBulkTasks = nil

	if msg.Task != nil && msg.Project != nil {
		m.showView(ViewSnoozeInput, NewSnoozeInputModel(msg.Task, msg.Project))
	}
	return true, nil
}

// handleSnoozeDateChanged handles the SnoozeDateChangedMsg.
func (m *AppModel) handleSnoozeDateChanged(msg SnoozeDateChangedMsg) (bool, tea.Cmd) {
	m.popView()

	refs := validTaskProjectRefs(m.pendingBulkTasks)
	if len(refs) > 0 && msg.HasExplicitValue {
		// Multiselect snooze picker (explicit value): commit every selected task's
		// snooze as one asynchronous cross-project transaction through the
		// BulkCoordinator. Snooze is an attribute mutation, so no workflow rules run;
		// the shell owns reload and selection-clear via the batch result.
		taskIDs := taskIDsFromTaskRefs(refs)
		m.pendingBulkTasks = nil
		var summary string
		if msg.SnoozedUntil != nil {
			summary = fmt.Sprintf("Snoozed %d tasks until %s", len(taskIDs), msg.SnoozedUntil.Format("2006-01-02"))
		} else {
			summary = fmt.Sprintf("Cleared snooze for %d tasks", len(taskIDs))
		}
		cmd := m.secondaryFeatures.Bulk.Snooze(taskIDs, msg.SnoozedUntil)
		return true, finalizeBatchResult(cmd, taskIDs, summary)
	}

	// Single-task snooze overlay (explicit value, no pending bulk selection):
	// commit the one task through the one-task BulkCoordinator wrapper so the
	// durable write flows through the application layer / workspace ledger instead
	// of a raw save. The shell owns reload and toast via the batch result.
	if len(refs) == 0 && msg.Task != nil && msg.HasExplicitValue {
		m.pendingBulkTasks = nil
		cmd := m.secondaryFeatures.Bulk.Snooze([]string{msg.Task.ID}, msg.SnoozedUntil)
		var summary string
		if msg.SnoozedUntil != nil {
			summary = fmt.Sprintf("Snoozed #%d until %s", msg.Task.ShortID, msg.SnoozedUntil.Format("2006-01-02"))
		} else {
			summary = fmt.Sprintf("Cleared snooze for #%d", msg.Task.ShortID)
		}
		return true, finalizeBatchResult(cmd, nil, summary)
	}

	if len(refs) > 1 && msg.Task != nil {
		// Non-explicit multiselect fallback: the snooze value already lives on the
		// first task (updated by SnoozeInputModel); apply it to every selected task
		// through the BulkCoordinator as one asynchronous cross-project transaction,
		// mirroring the explicit-value picker path above. The shell owns reload and
		// selection-clear via the batch result.
		taskIDs := taskIDsFromTaskRefs(refs)
		m.pendingBulkTasks = nil
		newSnooze := msg.Task.SnoozedUntil
		cmd := m.secondaryFeatures.Bulk.Snooze(taskIDs, newSnooze)
		var summary string
		if newSnooze != nil {
			summary = fmt.Sprintf("Snoozed %d tasks until %s", len(taskIDs), newSnooze.Format("2006-01-02"))
		} else {
			summary = fmt.Sprintf("Cleared snooze for %d tasks", len(taskIDs))
		}
		return true, finalizeBatchResult(cmd, taskIDs, summary)
	}

	m.pendingBulkTasks = nil
	m.refreshAllViews()
	return true, nil
}

// handleDueDateChanged handles the DueDateChangedMsg, including bulk operations.
func (m *AppModel) handleDueDateChanged(msg DueDateChangedMsg) (bool, tea.Cmd) {
	m.popView()

	// Check if this is a bulk operation from palette
	refs := validTaskProjectRefs(m.pendingBulkTasks)
	if len(refs) > 0 && msg.HasExplicitValue {
		// Multiselect due-date picker (explicit value): commit every selected task's
		// due date as one asynchronous cross-project transaction through the
		// BulkCoordinator. Date-only vs timed semantics ride msg.HasTime. Setting a
		// due date is an attribute mutation, so no workflow rules run; the shell owns
		// reload and selection-clear via the batch result.
		taskIDs := taskIDsFromTaskRefs(refs)
		m.pendingBulkTasks = nil
		var cmd tea.Cmd
		var summary string
		if msg.DueDate != nil {
			cmd = m.secondaryFeatures.Bulk.SetDueDate(taskIDs, *msg.DueDate, msg.HasTime)
			summary = fmt.Sprintf("Set due date to %s for %d tasks", msg.DueDate.Format("2006-01-02"), len(taskIDs))
		} else {
			cmd = m.secondaryFeatures.Bulk.ClearDueDate(taskIDs)
			summary = fmt.Sprintf("Cleared due date for %d tasks", len(taskIDs))
		}
		return true, finalizeBatchResult(cmd, taskIDs, summary)
	}

	// Single-task due-date overlay (explicit value, no pending bulk selection):
	// commit the one task through the one-task BulkCoordinator wrapper so the
	// durable write flows through the application layer / workspace ledger instead
	// of a raw save. Date-only vs timed semantics ride msg.HasTime; a nil DueDate
	// clears the date. The shell owns reload and toast via the batch result.
	if len(refs) == 0 && msg.Task != nil && msg.HasExplicitValue {
		m.pendingBulkTasks = nil
		var cmd tea.Cmd
		var summary string
		if msg.DueDate != nil {
			cmd = m.secondaryFeatures.Bulk.SetDueDate([]string{msg.Task.ID}, *msg.DueDate, msg.HasTime)
			summary = fmt.Sprintf("Set due date for #%d", msg.Task.ShortID)
		} else {
			cmd = m.secondaryFeatures.Bulk.ClearDueDate([]string{msg.Task.ID})
			summary = fmt.Sprintf("Cleared due date for #%d", msg.Task.ShortID)
		}
		return true, finalizeBatchResult(cmd, nil, summary)
	}

	if len(refs) > 1 && msg.Task != nil {
		// Non-explicit multiselect fallback: the due date already lives on the first
		// task (updated by DateInputModel); apply it to every selected task through
		// the BulkCoordinator as one asynchronous cross-project transaction, mirroring
		// the explicit-value picker path above. Date-only vs timed semantics ride the
		// first task's HasTime flag. The shell owns reload and selection-clear via the
		// batch result.
		taskIDs := taskIDsFromTaskRefs(refs)
		m.pendingBulkTasks = nil
		newDate := msg.Task.DueDate
		newHasTime := msg.Task.HasTime
		var cmd tea.Cmd
		var summary string
		if newDate != nil {
			cmd = m.secondaryFeatures.Bulk.SetDueDate(taskIDs, *newDate, newHasTime)
			summary = fmt.Sprintf("Set due date to %s for %d tasks", newDate.Format("2006-01-02"), len(taskIDs))
		} else {
			cmd = m.secondaryFeatures.Bulk.ClearDueDate(taskIDs)
			summary = fmt.Sprintf("Cleared due date for %d tasks", len(taskIDs))
		}
		return true, finalizeBatchResult(cmd, taskIDs, summary)
	}

	m.pendingBulkTasks = nil
	m.refreshAllViews()
	return true, nil
}

func taskProjectRefsFromTasks(tasks []*models.Task, project *models.Project) []TaskProjectRef {
	if project == nil {
		return nil
	}
	refs := make([]TaskProjectRef, 0, len(tasks))
	for _, task := range tasks {
		if task == nil {
			continue
		}
		refs = append(refs, TaskProjectRef{Task: task, Project: project})
	}
	return refs
}

func validTaskProjectRefs(refs []TaskProjectRef) []TaskProjectRef {
	valid := make([]TaskProjectRef, 0, len(refs))
	for _, ref := range refs {
		if ref.Task == nil || ref.Project == nil {
			continue
		}
		valid = append(valid, ref)
	}
	return valid
}
