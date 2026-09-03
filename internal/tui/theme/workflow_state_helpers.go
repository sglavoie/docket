package theme

import "github.com/sglavoie/tt/internal/models"

// TaskStateGroupLabel returns the canonical presentation group for a task.
func TaskStateGroupLabel(t *models.Task) string {
	if t == nil {
		return ""
	}
	return models.WorkflowStateGroupForState(t.State()).Label()
}

// TaskStateGroup returns the canonical presentation group derived from task
// state.
func TaskStateGroup(t *models.Task) models.WorkflowStateGroup {
	if t == nil {
		return ""
	}
	return models.WorkflowStateGroupForState(t.State())
}
