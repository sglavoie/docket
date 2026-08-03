package tui

import "github.com/sglavoie/tt/internal/models"

// taskStateGroupLabel returns the canonical presentation group for a task.
func taskStateGroupLabel(t *models.Task) string {
	if t == nil {
		return ""
	}
	return models.WorkflowStateGroupForState(t.State()).Label()
}

// taskStateGroup returns the canonical presentation group derived from task
// state.
func taskStateGroup(t *models.Task) models.WorkflowStateGroup {
	if t == nil {
		return ""
	}
	return models.WorkflowStateGroupForState(t.State())
}
