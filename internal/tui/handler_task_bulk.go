package tui

import (
	"fmt"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
)

// handleCommandPaletteBulkAction handles bulk actions on multiple selected tasks.
func (m *AppModel) handleCommandPaletteBulkAction(msg CommandPaletteBulkActionMsg) (bool, tea.Cmd) {
	if len(msg.Tasks) == 0 {
		return true, nil
	}

	refs := validTaskProjectRefs(msg.Tasks)

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

	// Apply action to all tasks. Gate the transition check outside MutateTask
	// so skipped tasks neither inflate the count nor have their UpdatedAt bumped.
	count, _, err := m.applyBulkTaskRefsWithWorkflow(refs, StatusVerb(targetStatus)+" Task",
		func(project *models.Project, taskID string) bool {
			task := project.GetTaskByID(taskID)
			if task == nil || task.CanTransitionTo(targetStatus) != "" {
				return false
			}
			return project.MutateTask(taskID, func(t *models.Task) {
				t.ApplyTransition(targetStatus)
			})
		})
	if err != nil {
		return true, errorCmd("save failed: %w", err)
	}

	if count > 0 {
		m.clearBulkTaskSelectionsForRefs(refs)
	}

	if count == 0 {
		m.setInfoStatus("No tasks were changed")
		return true, defaultStatusTimeout()
	}

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
	m.setSuccessStatus(successMsg)
	cmds := m.refreshTaskViewsAfterSave()
	cmds = append(cmds, defaultStatusTimeout())
	return true, tea.Batch(cmds...)
}
