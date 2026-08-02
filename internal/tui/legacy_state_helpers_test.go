package tui

import "github.com/sglavoie/tt/internal/models"

// legacyStateForStatusSection collapses a legacy (status, section) pair onto a
// single canonical WorkflowState, applying the migration precedence used across
// the refactored tests: canceled > done/Completed > in_progress >
// blocked/Waiting > standby/Standby > maybe/Someday > Inbox > Next > ready.
// Earlier rules win when multiple legacy signals are present.
func legacyStateForStatusSection(status models.WorkflowState, section string) models.WorkflowState {
	switch {
	case status == models.StateCanceled:
		return models.StateCanceled
	case status == models.StateDone || section == string(models.StateDone):
		return models.StateDone
	case status == models.StateDoing:
		return models.StateDoing
	case status == models.StateWaiting || section == string(models.StateWaiting):
		return models.StateWaiting
	case status == models.StatePaused || section == string(models.StatePaused):
		return models.StatePaused
	case status == models.StateSomeday || section == string(models.StateSomeday):
		return models.StateSomeday
	case section == string(models.StateInbox):
		return models.StateInbox
	case section == string(models.StateBacklog):
		return models.StateBacklog
	default:
		return models.StateBacklog
	}
}
