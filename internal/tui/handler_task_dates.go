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
		weekEndDay := ""
		if m.config != nil {
			weekEndDay = m.config.GetWeekEndDay()
		}
		input := NewDateInputModel(first.Task, first.Project, weekEndDay)
		input.deferApply = true
		m.showView(ViewDateInput, input)
		return true, nil
	}
	m.pendingBulkTasks = nil

	if msg.Task != nil && msg.Project != nil {
		weekEndDay := ""
		if m.config != nil {
			weekEndDay = m.config.GetWeekEndDay()
		}
		m.showView(ViewDateInput, NewDateInputModel(msg.Task, msg.Project, weekEndDay))
	}
	return true, nil
}

// handleSetSnoozeDate handles the SetSnoozeDateMsg.
func (m *AppModel) handleSetSnoozeDate(msg SetSnoozeDateMsg) (bool, tea.Cmd) {
	if refs := validTaskProjectRefs(msg.TaskRefs); len(refs) > 0 {
		m.pendingBulkTasks = refs
		first := refs[0]
		input := NewSnoozeInputModel(first.Task, first.Project)
		input.deferApply = true
		m.showView(ViewSnoozeInput, input)
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
		newSnooze := msg.SnoozedUntil
		count, _, err := m.applyBulkTaskRefsWithWorkflow(refs, "Snooze", func(project *models.Project, taskID string) bool {
			return project.MutateTask(taskID, func(t *models.Task) {
				if newSnooze != nil {
					snoozeCopy := *newSnooze
					t.SnoozedUntil = &snoozeCopy
				} else {
					t.SnoozedUntil = nil
				}
			})
		})
		m.pendingBulkTasks = nil
		if err != nil {
			return true, errorCmd("save failed: %w", err)
		}

		var successMsg string
		if newSnooze != nil {
			successMsg = fmt.Sprintf("Snoozed %d tasks until %s", count, newSnooze.Format("2006-01-02"))
		} else {
			successMsg = fmt.Sprintf("Cleared snooze for %d tasks", count)
		}
		m.clearBulkTaskSelectionsForRefs(refs)
		m.setSuccessStatus(successMsg)
		cmds := m.refreshTaskViewsAfterSave()
		cmds = append(cmds, defaultStatusTimeout())
		return true, tea.Batch(cmds...)
	}

	if len(refs) > 1 && msg.Task != nil {
		newSnooze := msg.Task.SnoozedUntil
		count := 1 // First task was already updated and saved by SnoozeInputModel.

		for i, ref := range refs {
			if i == 0 {
				continue
			}
			if ref.Project.MutateTask(ref.Task.ID, func(t *models.Task) {
				if newSnooze != nil {
					snoozeCopy := *newSnooze
					t.SnoozedUntil = &snoozeCopy
				} else {
					t.SnoozedUntil = nil
				}
			}) {
				count++
			}
		}

		var successMsg string
		if newSnooze != nil {
			successMsg = fmt.Sprintf("Snoozed %d tasks until %s", count, newSnooze.Format("2006-01-02"))
		} else {
			successMsg = fmt.Sprintf("Cleared snooze for %d tasks", count)
		}
		m.pendingBulkTasks = nil
		m.clearBulkTaskSelectionsForRefs(refs)

		if cmd := saveAffectedProjectsAsync(refs, successMsg); cmd != nil {
			return true, cmd
		}
		m.setSuccessStatus(successMsg)
		m.refreshAllViews()
		return true, defaultStatusTimeout()
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
		newDate := msg.DueDate
		newHasTime := msg.HasTime
		count, _, err := m.applyBulkTaskRefsWithWorkflow(refs, "Set Due Date", func(project *models.Project, taskID string) bool {
			return project.MutateTask(taskID, func(t *models.Task) {
				if newDate != nil {
					dateCopy := *newDate
					t.DueDate = &dateCopy
				} else {
					t.DueDate = nil
				}
				t.HasTime = newHasTime
			})
		})
		m.pendingBulkTasks = nil
		if err != nil {
			return true, errorCmd("save failed: %w", err)
		}

		var successMsg string
		if newDate != nil {
			successMsg = fmt.Sprintf("Set due date to %s for %d tasks", newDate.Format("2006-01-02"), count)
		} else {
			successMsg = fmt.Sprintf("Cleared due date for %d tasks", count)
		}
		m.clearBulkTaskSelectionsForRefs(refs)
		m.setSuccessStatus(successMsg)
		cmds := m.refreshTaskViewsAfterSave()
		cmds = append(cmds, defaultStatusTimeout())
		return true, tea.Batch(cmds...)
	}

	if len(refs) > 1 && msg.Task != nil {
		// The first task was already updated and saved by DateInputModel.
		// Apply the same date to remaining tasks.
		newDate := msg.Task.DueDate
		newHasTime := msg.Task.HasTime
		count := 1

		for i, item := range refs {
			if i == 0 {
				continue
			}
			if item.Project.MutateTask(item.Task.ID, func(t *models.Task) {
				if newDate != nil {
					dateCopy := *newDate
					t.DueDate = &dateCopy
				} else {
					t.DueDate = nil
				}
				t.HasTime = newHasTime
			}) {
				count++
			}
		}

		var successMsg string
		if newDate != nil {
			successMsg = fmt.Sprintf("Set due date to %s for %d tasks", newDate.Format("2006-01-02"), count)
		} else {
			successMsg = fmt.Sprintf("Cleared due date for %d tasks", count)
		}
		m.pendingBulkTasks = nil
		m.clearBulkTaskSelectionsForRefs(refs)

		if cmd := saveAffectedProjectsAsync(refs, successMsg); cmd != nil {
			return true, cmd
		}
		m.setSuccessStatus(successMsg)
		m.refreshAllViews()
		return true, defaultStatusTimeout()
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
