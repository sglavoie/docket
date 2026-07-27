package tui

import "github.com/sglavoie/tt/internal/models"

// OutdentTask attempts to outdent a task to its parent's level.
// Returns true if the outdent was performed.
func OutdentTask(project *models.Project, task *models.Task) bool {
	if ok, _ := task.CanOutdent(); !ok {
		return false
	}
	_ = project.OutdentTask(task)
	return true
}
