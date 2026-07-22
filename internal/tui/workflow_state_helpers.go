package tui

import "github.com/sglavoie/tt/internal/models"

// stateMatchesLegacyStatus maps the legacy status filter vocabulary onto the
// canonical workflow states.
func stateMatchesLegacyStatus(state models.WorkflowState, status models.TaskStatus) bool {
	if status == models.StatusTodo {
		return state == models.StateInbox || state == models.StateReady || state == models.StateNext
	}
	return state == models.TargetStateForStatus(status)
}

// taskInPlanningState reports whether the task sits in one of the three
// todo-ish planning states.
func taskInPlanningState(t *models.Task) bool {
	switch t.State() {
	case models.StateInbox, models.StateReady, models.StateNext:
		return true
	case models.StateInProgress, models.StateWaiting, models.StatePaused,
		models.StateReview, models.StateSomeday, models.StateDone, models.StateCanceled:
		return false
	}
	return false
}

// taskIsActionableState reports whether a task is immediately workable.
func taskIsActionableState(t *models.Task, p *models.Project) bool {
	if t.IsDependencyBlocked(p) {
		return false
	}
	state := t.State()
	return state == models.StateReady || state == models.StateInProgress
}

// taskSectionBucket returns the GTD section bucket for a task's workflow state.
func taskSectionBucket(t *models.Task) string {
	return models.SectionForState(t.State())
}

// stateForSectionName maps a GTD section name to its workflow state.
func stateForSectionName(section string) (models.WorkflowState, bool) {
	switch section {
	case "":
		return models.StateReady, true
	case models.SectionNext:
		return models.StateNext, true
	case models.SectionInbox:
		return models.StateInbox, true
	case models.SectionWaiting:
		return models.StateWaiting, true
	case models.SectionReview:
		return models.StateReview, true
	case models.SectionSomeday:
		return models.StateSomeday, true
	case models.SectionStandby:
		return models.StatePaused, true
	case models.SectionCompleted:
		return models.StateDone, true
	}
	return "", false
}

// legacyStatusColumnForState retains the historical status vocabulary used by
// saved smart-filter criteria and palette matching.
func legacyStatusColumnForState(state models.WorkflowState) string {
	switch state {
	case models.StateInbox, models.StateReady, models.StateNext:
		return "todo"
	case models.StateInProgress:
		return "in_progress"
	case models.StateWaiting:
		return "blocked"
	case models.StatePaused:
		return "standby"
	case models.StateReview:
		return "review"
	case models.StateSomeday:
		return "maybe"
	case models.StateDone:
		return "done"
	case models.StateCanceled:
		return "canceled"
	default:
		return string(state)
	}
}
