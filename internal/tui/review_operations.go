package tui

import (
	"strconv"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/sglavoie/tt/internal/models"
)

// reviewOpsView abstracts the view-specific operations needed by shared review
// task operations. Both DailyReviewModel and WeeklyReviewModel implement this.
type reviewOpsView interface {
	getReviewTaskRef() (task *models.Task, project *models.Project)
	reloadReviewData()
	clampCursor()
	setReviewErr(error)
	setReviewStatus(string)
}

// reviewApplyOp runs a shared task operation against the current cursor task,
// applies the result to the view, reloads, and returns the command.
func reviewApplyOp(v reviewOpsView, op func(project *models.Project, taskID string) TaskOpResult) tea.Cmd {
	task, project := v.getReviewTaskRef()
	if task == nil {
		return nil
	}

	result := op(project, task.ID)
	if result.Err != nil {
		v.setReviewErr(result.Err)
	} else if result.StatusMessage != "" {
		v.setReviewStatus(result.StatusMessage)
	}
	v.reloadReviewData()
	v.clampCursor()
	return result.Cmd
}

// reviewChangeStatus changes the status of the task at cursor.
func reviewChangeStatus(v reviewOpsView, newStatus models.TaskStatus) (tea.Model, tea.Cmd) {
	cmd := reviewApplyOp(v, func(project *models.Project, taskID string) TaskOpResult {
		return applyStatusChange(defaultPersister, project, taskID, newStatus)
	})
	return v.(tea.Model), cmd
}

// reviewChangePriority changes the priority of the task at cursor.
func reviewChangePriority(v reviewOpsView, key string) (tea.Model, tea.Cmd) {
	priority, _ := strconv.Atoi(key)
	cmd := reviewApplyOp(v, func(project *models.Project, taskID string) TaskOpResult {
		return applyPriorityChange(defaultPersister, project, taskID, priority)
	})
	return v.(tea.Model), cmd
}

// reviewSetQuickDueDate sets the due date to a relative number of days from today.
func reviewSetQuickDueDate(v reviewOpsView, daysFromNow int) (tea.Model, tea.Cmd) {
	cmd := reviewApplyOp(v, func(project *models.Project, taskID string) TaskOpResult {
		return applyQuickDueDate(defaultPersister, project, taskID, daysFromNow)
	})
	return v.(tea.Model), cmd
}

// reviewClearDueDate clears the due date from the task at cursor.
func reviewClearDueDate(v reviewOpsView) (tea.Model, tea.Cmd) {
	cmd := reviewApplyOp(v, func(project *models.Project, taskID string) TaskOpResult {
		return applyClearDueDate(defaultPersister, project, taskID)
	})
	return v.(tea.Model), cmd
}

// reviewToggleActive toggles the Active flag on the task at cursor.
func reviewToggleActive(v reviewOpsView) (tea.Model, tea.Cmd) {
	cmd := reviewApplyOp(v, func(project *models.Project, taskID string) TaskOpResult {
		return applyToggleActive(defaultPersister, project, taskID)
	})
	return v.(tea.Model), cmd
}

// reviewSectionMove moves the task at cursor to the specified GTD section.
func reviewSectionMove(v reviewOpsView, section string) (tea.Model, tea.Cmd) {
	cmd := reviewApplyOp(v, func(project *models.Project, taskID string) TaskOpResult {
		return applySectionChange(defaultPersister, project, taskID, section)
	})
	return v.(tea.Model), cmd
}

// reviewDeleteTask deletes the task at cursor.
func reviewDeleteTask(v reviewOpsView) (tea.Model, tea.Cmd) {
	cmd := reviewApplyOp(v, func(project *models.Project, taskID string) TaskOpResult {
		return applyDeleteTask(defaultPersister, project, taskID)
	})
	return v.(tea.Model), cmd
}
