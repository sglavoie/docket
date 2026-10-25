package app

import "github.com/sglavoie/docket/internal/models"

// newTaskForCreation is the sole live construction seam and always creates a
// generic task.
func newTaskForCreation(id string, shortID int, title string) (*models.Task, error) {
	return models.NewGenericTask(id, shortID, title)
}
