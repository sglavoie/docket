package tui

import (
	"github.com/sglavoie/tt/internal/models"
)

// makeTask creates a minimal task with the given title and section. The section
// name is mapped onto the task's canonical WorkflowState via SetSection.
func makeTask(id, title string, state models.WorkflowState) *models.Task {
	task := &models.Task{
		ID:    id,
		Title: title,
	}
	task.ApplyWorkflowState(state)
	return task
}
