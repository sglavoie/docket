package tui

import (
	"strconv"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
)

// taskOpsView abstracts the view-specific operations needed by shared task
// operations. Views that support task mutations (status, priority, due date,
// etc.) implement this interface so they can all share one set of helpers.
type taskOpsView interface {
	getTaskRef() (task *models.Task, project *models.Project)
	getPersister() Persister
	getConfig() *config.Config
	getConfigManager() *config.Manager
	getDataDir() string
	reloadData()
	clampCursor()
	setErr(error)
	setStatus(string)
}

// taskOpsApply runs a shared task operation against the current cursor task,
// applies the result to the view, reloads, and returns the command.
// It uses the full workflow pipeline (auto-unblock, recurring generation,
// workflow rules, root reorder) so secondary views get parity with primary views.
func taskOpsApply(v taskOpsView, op func(pf persistFn, project *models.Project, taskID string) TaskOpResult) tea.Cmd {
	task, project := v.getTaskRef()
	if task == nil {
		return nil
	}

	pf := workflowPersist(func() WorkflowSaveOptions {
		return BuildWorkflowOpts(v.getPersister(), project, v.getConfig(), v.getConfigManager(), nil, "", "")
	})
	result := op(pf, project, task.ID)
	if result.Err != nil {
		v.setErr(result.Err)
	} else if result.StatusMessage != "" {
		v.setStatus(result.StatusMessage)
	}
	v.reloadData()
	v.clampCursor()
	if result.StatusMessage != "" {
		return result.Cmd
	}
	return result.Cmd
}

// taskOpsChangeStatus changes the status of the task at cursor.
func taskOpsChangeStatus(v taskOpsView, newStatus models.TaskStatus) (tea.Model, tea.Cmd) {
	cmd := taskOpsApply(v, func(pf persistFn, project *models.Project, taskID string) TaskOpResult {
		return applyStatusChange(pf, project, taskID, newStatus)
	})
	return v.(tea.Model), cmd
}

// taskOpsChangePriority changes the priority of the task at cursor.
func taskOpsChangePriority(v taskOpsView, key string) (tea.Model, tea.Cmd) {
	priority, _ := strconv.Atoi(key)
	cmd := taskOpsApply(v, func(pf persistFn, project *models.Project, taskID string) TaskOpResult {
		return applyPriorityChange(pf, project, taskID, priority)
	})
	return v.(tea.Model), cmd
}

// taskOpsSetQuickDueDate sets the due date to a relative number of days from today.
func taskOpsSetQuickDueDate(v taskOpsView, daysFromNow int) (tea.Model, tea.Cmd) {
	cmd := taskOpsApply(v, func(pf persistFn, project *models.Project, taskID string) TaskOpResult {
		return applyQuickDueDate(pf, project, taskID, daysFromNow)
	})
	return v.(tea.Model), cmd
}

// taskOpsClearDueDate clears the due date from the task at cursor.
func taskOpsClearDueDate(v taskOpsView) (tea.Model, tea.Cmd) {
	cmd := taskOpsApply(v, func(pf persistFn, project *models.Project, taskID string) TaskOpResult {
		return applyClearDueDate(pf, project, taskID)
	})
	return v.(tea.Model), cmd
}

// taskOpsQuickSnooze sets the snooze date on the task at cursor.
func taskOpsQuickSnooze(v taskOpsView, snoozeDate *time.Time) (tea.Model, tea.Cmd) {
	cmd := taskOpsApply(v, func(pf persistFn, project *models.Project, taskID string) TaskOpResult {
		return applyQuickSnooze(pf, project, taskID, snoozeDate)
	})
	return v.(tea.Model), cmd
}

// taskOpsToggleActive toggles the Active flag on the task at cursor.
func taskOpsToggleActive(v taskOpsView) (tea.Model, tea.Cmd) {
	cmd := taskOpsApply(v, func(pf persistFn, project *models.Project, taskID string) TaskOpResult {
		return applyToggleActive(pf, project, taskID)
	})
	return v.(tea.Model), cmd
}

// taskOpsRemoveFromFocus always sets Active = false (FocusView-specific).
func taskOpsRemoveFromFocus(v taskOpsView, shortID int) (tea.Model, tea.Cmd) {
	cmd := taskOpsApply(v, func(pf persistFn, project *models.Project, taskID string) TaskOpResult {
		return applyRemoveFromFocus(pf, project, taskID, shortID)
	})
	return v.(tea.Model), cmd
}

// taskOpsSectionMove moves the task at cursor to the specified GTD section.
func taskOpsSectionMove(v taskOpsView, section string) (tea.Model, tea.Cmd) {
	cmd := taskOpsApply(v, func(pf persistFn, project *models.Project, taskID string) TaskOpResult {
		return applySectionChange(pf, project, taskID, section)
	})
	return v.(tea.Model), cmd
}

// taskOpsDeleteTask deletes the task at cursor.
func taskOpsDeleteTask(v taskOpsView) (tea.Model, tea.Cmd) {
	dataDir := v.getDataDir()
	cmd := taskOpsApply(v, func(pf persistFn, project *models.Project, taskID string) TaskOpResult {
		return applyDeleteTask(pf, project, taskID, dataDir)
	})
	return v.(tea.Model), cmd
}
