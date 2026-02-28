package tui

import (
	"time"

	"github.com/sglavoie/tt/internal/models"
)

// applyBulkPriority sets the priority on multiple tasks. Returns the count of
// tasks actually changed.
func applyBulkPriority(project *models.Project, tasks []*models.Task, priority int) int {
	count := 0
	for _, task := range tasks {
		if project.MutateTask(task.ID, func(t *models.Task) {
			t.Priority = priority
		}) {
			count++
		}
	}
	return count
}

// applyBulkDueDate sets (or clears) the due date on multiple tasks.
// Returns the count of tasks changed.
func applyBulkDueDate(project *models.Project, tasks []*models.Task, dueDate *time.Time) int {
	count := 0
	for _, task := range tasks {
		if project.MutateTask(task.ID, func(t *models.Task) {
			t.DueDate = dueDate
		}) {
			count++
		}
	}
	return count
}
