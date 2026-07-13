package tui

import (
	"fmt"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/util"
)

// taskOpsView implementation for DailyReviewModel.

func (m *DailyReviewModel) getTaskRef() (*models.Task, *models.Project) {
	dt := m.getTaskAtCursor()
	if dt == nil {
		return nil, nil
	}
	return dt.Task, dt.Project
}

func (m *DailyReviewModel) getPersister() Persister           { return defaultPersister }
func (m *DailyReviewModel) getConfigManager() *config.Manager { return m.configManager }
func (m *DailyReviewModel) reloadData()                       { m.loadTasks() }
func (m *DailyReviewModel) setErr(err error)                  { m.err = err }
func (m *DailyReviewModel) setStatus(msg string)              { m.InteractiveViewState.setStatus(msg) }

// applyReviewMutation commits a single-task review mutation through the
// application-layer single-task coordinator and advances review progress. The
// coordinator commits synchronously when cmd runs, so the on-disk change is
// durable before reloadData re-reads it; the returned transaction message flows
// to the shell for the shared post-save refresh (reloadData + clampCursor there
// too, harmless). A rejected mutation surfaces the error and leaves review
// progress untouched.
func (m *DailyReviewModel) applyReviewMutation(cmd tea.Cmd) tea.Cmd {
	if cmd == nil {
		return nil
	}
	msg := cmd()
	if em, ok := msg.(TransactionErrorMsg); ok {
		m.setErr(em.Err)
		return nil
	}
	m.reviewedCount++
	m.reloadData()
	m.clampCursor()
	if m.getTotalTaskCount() == 0 {
		m.setStatus("Daily review complete. All tasks reviewed.")
	}
	return func() tea.Msg { return msg }
}

// reviewStateBlockReason preflights the guards the coordinator would reject a
// workflow-state transition with, so an invalid transition surfaces a status
// message (not an error toast) and does not count as a review, matching the
// pre-migration behavior.
func reviewStateBlockReason(project *models.Project, task *models.Task, newState models.WorkflowState) string {
	if msg := models.CanTransitionState(task.State(), newState); msg != "" {
		return msg
	}
	if newState == models.StateDone && project.HasIncompleteDependencies(task) {
		return "cannot complete: has incomplete dependencies"
	}
	if newState.IsTerminal() {
		if incomplete := task.IncompleteChildren(); len(incomplete) > 0 {
			verb := "complete"
			if newState == models.StateCanceled {
				verb = "cancel"
			}
			return fmt.Sprintf("cannot %s #%d: has %d incomplete children", verb, task.ShortID, len(incomplete))
		}
	}
	return ""
}

// reviewChangeState transitions the cursor task to newState and increments reviewedCount.
func reviewChangeState(m *DailyReviewModel, newState models.WorkflowState) (tea.Model, tea.Cmd) {
	task, project := m.getTaskRef()
	if task == nil {
		return m, nil
	}
	if reason := reviewStateBlockReason(project, task, newState); reason != "" {
		m.setStatus(reason)
		return m, nil
	}
	return m, m.applyReviewMutation(m.taskItem.Transition(task.ID, newState))
}

// reviewChangePriority changes the priority of the cursor task and increments reviewedCount.
func reviewChangePriority(m *DailyReviewModel, key string) (tea.Model, tea.Cmd) {
	priority, _ := PriorityForPrefixKey(key)
	if priority == 0 {
		// Fall back for direct number keys
		for i, k := range []string{"1", "2", "3", "4", "5"} {
			if key == k {
				priority = i + 1
				break
			}
		}
	}
	task, _ := m.getTaskRef()
	if task == nil {
		return m, nil
	}
	return m, m.applyReviewMutation(m.taskItem.SetPriority(task.ID, priority))
}

// reviewSetQuickDueDate sets the due date and increments reviewedCount.
func reviewSetQuickDueDate(m *DailyReviewModel, daysFromNow int) (tea.Model, tea.Cmd) {
	task, _ := m.getTaskRef()
	if task == nil {
		return m, nil
	}
	due := util.StartOfDay(time.Now().AddDate(0, 0, daysFromNow))
	return m, m.applyReviewMutation(m.taskItem.SetDueDate(task.ID, &due, false))
}

// reviewClearDueDate clears the due date and increments reviewedCount.
func reviewClearDueDate(m *DailyReviewModel) (tea.Model, tea.Cmd) {
	task, _ := m.getTaskRef()
	if task == nil {
		return m, nil
	}
	if task.DueDate == nil {
		m.setStatus("Task has no due date")
		return m, nil
	}
	return m, m.applyReviewMutation(m.taskItem.SetDueDate(task.ID, nil, false))
}

// reviewQuickSnooze sets the snooze date and increments reviewedCount.
func reviewQuickSnooze(m *DailyReviewModel, snoozeDate *time.Time) (tea.Model, tea.Cmd) {
	task, _ := m.getTaskRef()
	if task == nil {
		return m, nil
	}
	return m, m.applyReviewMutation(m.taskItem.Snooze(task.ID, snoozeDate))
}

// reviewSectionMove moves the cursor task to a section and increments reviewedCount.
func reviewSectionMove(m *DailyReviewModel, section string) (tea.Model, tea.Cmd) {
	task, _ := m.getTaskRef()
	if task == nil {
		return m, nil
	}
	return m, m.applyReviewMutation(m.taskItem.SetSection(task.ID, section))
}
