package focus

import "github.com/sglavoie/tt/internal/models"

// PlanningTask wraps a task with its parent project for ordered cross-project planning.
type PlanningTask struct {
	Task       *models.Task
	Project    *models.Project
	TreePrefix string
}
