package tui

import (
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
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

// taskOpsApplyReview is a wrapper around taskOpsApply that increments
// reviewedCount on any successful mutation (no error).
func taskOpsApplyReview(m *DailyReviewModel, undoAction string, op func(pf persistFn, project *models.Project, taskID string) TaskOpResult) tea.Cmd {
	task, project := m.getTaskRef()
	if task == nil {
		return nil
	}
	undoCtxStr := task.Title
	pf := workflowPersist(func() WorkflowSaveOptions {
		return BuildWorkflowOpts(m.getPersister(), project, m.getConfig(), m.getConfigManager(), nil, undoAction, undoCtxStr, []string{task.ID})
	})
	result := op(pf, project, task.ID)
	if result.Err != nil {
		m.setErr(result.Err)
	} else {
		if result.StatusMessage != "" {
			m.setStatus(result.StatusMessage)
		}
		m.reviewedCount++
	}
	m.reloadData()
	m.clampCursor()
	if m.getTotalTaskCount() == 0 && result.Err == nil {
		m.setStatus("Daily review complete. All tasks reviewed.")
	}
	return result.Cmd
}

// reviewChangeStatus changes the status of the cursor task and increments reviewedCount.
func reviewChangeStatus(m *DailyReviewModel, newStatus models.TaskStatus) (tea.Model, tea.Cmd) {
	cmd := taskOpsApplyReview(m, StatusVerb(newStatus)+" Task", func(pf persistFn, project *models.Project, taskID string) TaskOpResult {
		return applyStatusChange(pf, project, taskID, newStatus)
	})
	return m, cmd
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
	p := priority
	cmd := taskOpsApplyReview(m, "Set Priority", func(pf persistFn, project *models.Project, taskID string) TaskOpResult {
		return applyPriorityChange(pf, project, taskID, p)
	})
	return m, cmd
}

// reviewChangeStage changes the stage of the cursor task and increments reviewedCount.
func reviewChangeStage(m *DailyReviewModel, stage string) (tea.Model, tea.Cmd) {
	cmd := taskOpsApplyReview(m, "Set Stage", func(pf persistFn, project *models.Project, taskID string) TaskOpResult {
		return applyStageChange(pf, project, taskID, stage)
	})
	return m, cmd
}

// reviewSetQuickDueDate sets the due date and increments reviewedCount.
func reviewSetQuickDueDate(m *DailyReviewModel, daysFromNow int) (tea.Model, tea.Cmd) {
	cmd := taskOpsApplyReview(m, "Set Due Date", func(pf persistFn, project *models.Project, taskID string) TaskOpResult {
		return applyQuickDueDate(pf, project, taskID, daysFromNow)
	})
	return m, cmd
}

// reviewClearDueDate clears the due date and increments reviewedCount.
func reviewClearDueDate(m *DailyReviewModel) (tea.Model, tea.Cmd) {
	cmd := taskOpsApplyReview(m, "Clear Due Date", func(pf persistFn, project *models.Project, taskID string) TaskOpResult {
		return applyClearDueDate(pf, project, taskID)
	})
	return m, cmd
}

// reviewQuickSnooze sets the snooze date and increments reviewedCount.
func reviewQuickSnooze(m *DailyReviewModel, snoozeDate *time.Time) (tea.Model, tea.Cmd) {
	cmd := taskOpsApplyReview(m, "Snooze", func(pf persistFn, project *models.Project, taskID string) TaskOpResult {
		return applyQuickSnooze(pf, project, taskID, snoozeDate)
	})
	return m, cmd
}

// reviewSectionMove moves the cursor task to a section and increments reviewedCount.
func reviewSectionMove(m *DailyReviewModel, section string) (tea.Model, tea.Cmd) {
	cmd := taskOpsApplyReview(m, "Move to Section", func(pf persistFn, project *models.Project, taskID string) TaskOpResult {
		return applySectionChange(pf, project, taskID, section)
	})
	return m, cmd
}
