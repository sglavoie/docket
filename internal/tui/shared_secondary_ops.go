package tui

import (
	"strconv"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/sglavoie/tt/internal/models"
)

// secondaryOpsView abstracts the view-specific operations needed by shared
// secondary task operations. Both AllTasksViewModel and FocusViewModel implement this.
type secondaryOpsView interface {
	getSecondaryTaskRef() (task *models.Task, project *models.Project)
	getSecondaryPersister() Persister
	reloadSecondaryData()
	clampSecondaryCursor()
	setSecondaryErr(error)
	setSecondaryStatus(string)
}

// secondaryApplyOp runs a shared task operation against the current cursor task,
// applies the result to the view, reloads, and returns the command.
func secondaryApplyOp(v secondaryOpsView, op func(p Persister, project *models.Project, taskID string) TaskOpResult) tea.Cmd {
	task, project := v.getSecondaryTaskRef()
	if task == nil {
		return nil
	}

	result := op(v.getSecondaryPersister(), project, task.ID)
	if result.Err != nil {
		v.setSecondaryErr(result.Err)
	} else if result.StatusMessage != "" {
		v.setSecondaryStatus(result.StatusMessage)
	}
	v.reloadSecondaryData()
	v.clampSecondaryCursor()
	return result.Cmd
}

// secondaryChangeStatus changes the status of the task at cursor.
func secondaryChangeStatus(v secondaryOpsView, newStatus models.TaskStatus) (tea.Model, tea.Cmd) {
	cmd := secondaryApplyOp(v, func(p Persister, project *models.Project, taskID string) TaskOpResult {
		return applyStatusChange(p, project, taskID, newStatus)
	})
	return v.(tea.Model), cmd
}

// secondaryChangePriority changes the priority of the task at cursor.
func secondaryChangePriority(v secondaryOpsView, key string) (tea.Model, tea.Cmd) {
	priority, _ := strconv.Atoi(key)
	cmd := secondaryApplyOp(v, func(p Persister, project *models.Project, taskID string) TaskOpResult {
		return applyPriorityChange(p, project, taskID, priority)
	})
	return v.(tea.Model), cmd
}

// secondarySetQuickDueDate sets the due date to a relative number of days from today.
func secondarySetQuickDueDate(v secondaryOpsView, daysFromNow int) (tea.Model, tea.Cmd) {
	cmd := secondaryApplyOp(v, func(p Persister, project *models.Project, taskID string) TaskOpResult {
		return applyQuickDueDate(p, project, taskID, daysFromNow)
	})
	return v.(tea.Model), cmd
}

// secondaryClearDueDate clears the due date from the task at cursor.
func secondaryClearDueDate(v secondaryOpsView) (tea.Model, tea.Cmd) {
	cmd := secondaryApplyOp(v, func(p Persister, project *models.Project, taskID string) TaskOpResult {
		return applyClearDueDate(p, project, taskID)
	})
	return v.(tea.Model), cmd
}

// secondaryToggleActive toggles the Active flag on the task at cursor.
func secondaryToggleActive(v secondaryOpsView) (tea.Model, tea.Cmd) {
	cmd := secondaryApplyOp(v, func(p Persister, project *models.Project, taskID string) TaskOpResult {
		return applyToggleActive(p, project, taskID)
	})
	return v.(tea.Model), cmd
}

// secondaryRemoveFromFocus always sets Active = false (FocusView-specific).
func secondaryRemoveFromFocus(v secondaryOpsView, shortID int) (tea.Model, tea.Cmd) {
	cmd := secondaryApplyOp(v, func(p Persister, project *models.Project, taskID string) TaskOpResult {
		return applyRemoveFromFocus(p, project, taskID, shortID)
	})
	return v.(tea.Model), cmd
}

// secondarySectionMove moves the task at cursor to the specified GTD section.
func secondarySectionMove(v secondaryOpsView, section string) (tea.Model, tea.Cmd) {
	cmd := secondaryApplyOp(v, func(p Persister, project *models.Project, taskID string) TaskOpResult {
		return applySectionChange(p, project, taskID, section)
	})
	return v.(tea.Model), cmd
}

// secondaryDeleteTask deletes the task at cursor.
func secondaryDeleteTask(v secondaryOpsView) (tea.Model, tea.Cmd) {
	cmd := secondaryApplyOp(v, func(p Persister, project *models.Project, taskID string) TaskOpResult {
		return applyDeleteTask(p, project, taskID)
	})
	return v.(tea.Model), cmd
}
