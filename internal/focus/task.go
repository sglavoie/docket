package focus

import "github.com/sglavoie/tt/internal/models"

// FocusTask wraps a task with its parent project for cross-project display.
type FocusTask struct {
	Task       *models.Task
	Project    *models.Project
	TreePrefix string
}
