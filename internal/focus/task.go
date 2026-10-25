package focus

import "github.com/sglavoie/docket/internal/models"

// PlanningTask wraps a task with its parent project for ordered cross-project planning.
type PlanningTask struct {
	Task    *models.Task
	Project *models.Project
}
