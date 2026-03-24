package tui

import (
	"fmt"
	"strings"

	"github.com/sglavoie/tt/internal/models"
)

// appendComment trims text, resolves the task by ID in project, appends a
// timestamped comment, and returns the updated task.
// Persistence is the caller's responsibility.
func appendComment(project *models.Project, taskID string, text string) (*models.Task, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil, fmt.Errorf("comment text cannot be empty")
	}
	task := project.GetTaskByID(taskID)
	if task == nil {
		return nil, fmt.Errorf("task not found")
	}
	task.AddComment(text)
	return task, nil
}
