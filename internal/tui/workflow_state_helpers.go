package tui

import (
	"github.com/sglavoie/tt/internal/config"
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

// TaskInFocusQueue reports whether the task is a member of the persisted
// Focus Queue. It replaces reads of the deprecated Task.Active flag:
// focus-queue membership is the canonical "active" signal under the workflow
// state model (the Active field is deleted with the other legacy fields in
// session 33.7). The queue is small, so a linear scan avoids allocating a
// membership set at per-row render sites.
func TaskInFocusQueue(cfg *config.Config, taskID string) bool {
	if cfg == nil {
		return false
	}
	for _, item := range cfg.GetFocusQueue() {
		if item.TaskID == taskID {
			return true
		}
	}
	return false
}
