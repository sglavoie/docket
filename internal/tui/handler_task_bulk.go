package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/sglavoie/tt/internal/models"
)

// handleCommandPaletteBulkAction handles bulk actions on multiple selected tasks.
func (m *AppModel) handleCommandPaletteBulkAction(msg CommandPaletteBulkActionMsg) (bool, tea.Cmd) {
	if len(msg.Tasks) == 0 {
		return true, nil
	}

	// Group tasks by project for efficient saving
	projectTasks := make(map[string][]struct {
		Task    *models.Task
		Project *models.Project
	})
	for _, item := range msg.Tasks {
		if item.Task != nil && item.Project != nil {
			projectTasks[item.Project.ID] = append(projectTasks[item.Project.ID], item)
		}
	}

	// Map action name to target status
	var targetStatus models.TaskStatus
	switch msg.Action {
	case "complete":
		targetStatus = models.StatusDone
	case "start":
		targetStatus = models.StatusInProgress
	case "review":
		targetStatus = models.StatusReview
	default:
		return true, nil
	}

	// Apply action to all tasks
	count := 0
	var projects []*models.Project
	for _, items := range projectTasks {
		for _, item := range items {
			mutated := item.Project.MutateTask(item.Task.ID, func(t *models.Task) {
				if t.CanTransitionTo(targetStatus) != "" {
					return
				}
				t.ApplyTransition(targetStatus)
			})
			if mutated {
				count++
			}
		}
		if len(items) > 0 {
			projects = append(projects, items[0].Project)
		}
	}

	// Build success message
	actionPast := msg.Action + "d"
	switch msg.Action {
	case "complete":
		actionPast = "completed"
	case "start":
		actionPast = "started"
	case "review":
		actionPast = "sent to review"
	}
	successMsg := fmt.Sprintf("%d tasks %s", count, actionPast)

	// Async save with loading indicator
	return true, tea.Batch(
		startLoading("Saving..."),
		SaveMultipleProjectsAsync(projects, successMsg),
	)
}
