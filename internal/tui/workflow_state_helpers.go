package tui

import "github.com/sglavoie/tt/internal/models"

// taskIsActionableState reports whether a task is immediately workable.
func taskIsActionableState(t *models.Task, p *models.Project) bool {
	if t.IsDependencyBlocked(p) {
		return false
	}
	state := t.State()
	return state == models.StateReady || state == models.StateInProgress
}

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
