package tui

import (
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/testutil"
)

func makeTestCard(section string, status models.TaskStatus) KanbanCard {
	task := testutil.NewTestTask("task", models.TaskTypeTask)
	task.WorkflowState = workflowStateForSectionStatus(section, status)
	proj := testutil.NewTestProject("p")
	return KanbanCard{Task: task, Project: proj}
}

// workflowStateForSectionStatus collapses a legacy (section, status) pair into
// the single canonical WorkflowState using the retired-field precedence:
// canceled > done/Completed > review/Review > in_progress > blocked/Waiting >
// standby/Standby > maybe/Someday > Inbox > Next > ready.
func workflowStateForSectionStatus(section string, status models.TaskStatus) models.WorkflowState {
	switch {
	case status == models.StatusCanceled:
		return models.StateCanceled
	case status == models.StatusDone || section == models.SectionCompleted:
		return models.StateDone
	case status == models.StatusReview || section == models.SectionReview:
		return models.StateReview
	case status == models.StatusInProgress:
		return models.StateInProgress
	case status == models.StatusBlocked || section == models.SectionWaiting:
		return models.StateWaiting
	case status == models.StatusStandby || section == models.SectionStandby:
		return models.StatePaused
	case status == models.StatusMaybe || section == models.SectionSomeday:
		return models.StateSomeday
	case section == models.SectionInbox:
		return models.StateInbox
	case section == models.SectionNext:
		return models.StateNext
	default:
		return models.StateReady
	}
}
