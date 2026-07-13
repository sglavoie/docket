package tui

import "github.com/sglavoie/tt/internal/models"

// legacyStateForStatusSection collapses a legacy (status, section) pair onto a
// single canonical WorkflowState, applying the migration precedence used across
// the refactored tests: canceled > done/Completed > review/Review >
// in_progress > blocked/Waiting > standby/Standby > maybe/Someday > Inbox >
// Next > ready. Earlier rules win when multiple legacy signals are present.
func legacyStateForStatusSection(status models.TaskStatus, section string) models.WorkflowState {
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
