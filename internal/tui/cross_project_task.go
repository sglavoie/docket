package tui

import "github.com/sglavoie/tt/internal/models"

// CrossProjectTask wraps a task with its parent project for cross-project display.
// Used by daily review, due date overview, all tasks, and recent activity views.
type CrossProjectTask struct {
	Task    *models.Task
	Project *models.Project
}
