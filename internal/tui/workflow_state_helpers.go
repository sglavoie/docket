package tui

import (
	"github.com/sglavoie/tt/internal/models"
)

// stateMatchesLegacyStatus maps the legacy status filter vocabulary onto the
// canonical workflow states, mirroring cmd/task_list.go's mapping so CLI and
// TUI status filters agree: each status matches the state it transitions to
// (models.TargetStateForStatus), except todo, which covers all three todo-ish
// planning states (inbox, ready, next). Under the state collapse a legacy
// Waiting-section todo task matches blocked (waiting), not todo.
//
// This adapter now backs only the free-form search-query language and the
// legacy saved-filter status predicate; the interactive transition controls,
// pickers, hints, and perspectives all read WorkflowState directly. Removed once
// those last two consumers migrate off the legacy status vocabulary.
func stateMatchesLegacyStatus(state models.WorkflowState, status models.TaskStatus) bool {
	if status == models.StatusTodo {
		return state == models.StateInbox || state == models.StateReady || state == models.StateNext
	}
	return state == models.TargetStateForStatus(status)
}

// taskInPlanningState reports whether the task sits in one of the three todo-ish
// planning states (inbox, ready, next). It is the state-model successor to the
// old "todo status" check used by palettes, perspectives, and reviews to select
// not-yet-started, non-holding work.
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

// taskIsActionableState reports whether a task is immediately workable: it
// sits in the ready or in_progress state and carries no unresolved
// dependencies. This is the state-model successor to the legacy "Actionable"
// filter predicate, which excluded the blocked status, every GTD holding
// section, and incomplete dependencies. Statuses that used to slip through
// with an empty section (review, standby, maybe) now map to their holding
// states and are excluded too.
func taskIsActionableState(t *models.Task, p *models.Project) bool {
	if t.IsDependencyBlocked(p) {
		return false
	}
	st := t.State()
	return st == models.StateReady || st == models.StateInProgress
}

// taskSectionBucket returns the GTD section bucket the task's workflow state
// maps to ("" for the active ready/in_progress bucket). Views that filter or
// group by section names read this instead of the legacy Section field.
func taskSectionBucket(t *models.Task) string {
	return models.SectionForState(t.State())
}

// Next (queue) membership derives from taskIsNext (Task.State()==StateNext); the
// Focus Queue order index is consulted only to order qualifying tasks (see
// focus.FocusQueueRank). There is no membership helper keyed on the order index.
