package tui

import (
	"fmt"
	"strings"

	"github.com/sglavoie/tt/internal/models"
)

// appendDecision trims text, resolves the task by ID in project, appends a
// timestamped decision, and returns the updated task.
// Persistence is the caller's responsibility.
func appendDecision(project *models.Project, taskID, text, rationale string, category models.DecisionCategory) (*models.Task, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil, fmt.Errorf("decision text cannot be empty")
	}
	task := project.GetTaskByID(taskID)
	if task == nil {
		return nil, fmt.Errorf("task not found")
	}
	task.AddDecision(text, rationale, category)
	return task, nil
}
