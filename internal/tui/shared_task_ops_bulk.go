package tui

import (
	"time"

	"github.com/sglavoie/tt/internal/models"
)

// applyBulkDueDate sets (or clears) the due date on multiple tasks.
// Returns the count of tasks changed.
func applyBulkDueDate(project *models.Project, tasks []*models.Task, dueDate *time.Time) int {
	count := 0
	for _, task := range tasks {
		if project.MutateTask(task.ID, func(t *models.Task) {
			if dueDate != nil {
				d := *dueDate // copy so each task owns its own *time.Time
				t.DueDate = &d
			} else {
				t.DueDate = nil
			}
		}) {
			count++
		}
	}
	return count
}
