package tui

import "github.com/sglavoie/tt/internal/models"

// CrossProjectTask wraps a task with its parent project for cross-project display.
type CrossProjectTask struct {
	Task    *models.Task
	Project *models.Project
}
