package models

import "strings"

// This file holds the surviving vocabulary that bridges the retired legacy task
// lifecycle values (status, section, stage) and the canonical WorkflowState
// model. The legacy fields no longer exist on Task: WorkflowState is the single
// stored source of truth. DeriveWorkflowState remains as a PURE loader-upgrade
// function — the storage parser calls it with the raw status/section/stage a
// schema-v1 document carried to upgrade it to a stored state.
// SectionForState/TargetStateForStatus/LegacyStatusForState keep the display and
// rule-matching vocabulary that some surfaces still speak in the legacy
// status/section terms, derived from State().

// DeriveWorkflowState maps the legacy lifecycle fields to the single canonical
// WorkflowState using the exact precedence table locked in context.md. Earlier
// rules win: a canceled status always resolves to canceled even if the section
// or stage would otherwise suggest something else.
//
// This is a PURE loader-upgrade function: it takes the raw status/section/stage
// a schema-v1 document carried and returns the state that document upgrades to.
// It reads no task fields.
//
// Precedence:
//  1. Canceled status                         -> canceled
//  2. Done status or Completed section        -> done
//  3. Review status/section/stage             -> review
//  4. In-progress status                      -> in_progress
//  5. Blocked status or Waiting section       -> waiting
//  6. Standby status or Standby section       -> paused
//  7. Maybe status or Someday section         -> someday
//  8. Inbox section                           -> inbox
//  9. Next section                            -> next
//  10. Everything else                        -> ready
//
// The legacy Active flag intentionally does not participate: it maps to Focus
// Queue membership at migration time, not to a workflow state.
func DeriveWorkflowState(status TaskStatus, section, stage string) WorkflowState {
	switch {
	case status == StatusCanceled:
		return StateCanceled
	case status == StatusDone || section == SectionCompleted:
		return StateDone
	case status == StatusReview || section == SectionReview || strings.EqualFold(stage, SectionReview):
		return StateReview
	case status == StatusInProgress:
		return StateInProgress
	case status == StatusBlocked || section == SectionWaiting:
		return StateWaiting
	case status == StatusStandby || section == SectionStandby:
		return StatePaused
	case status == StatusMaybe || section == SectionSomeday:
		return StateSomeday
	case section == SectionInbox:
		return StateInbox
	case section == SectionNext:
		return StateNext
	default:
		return StateReady
	}
}

// State returns the task's authoritative WorkflowState. The stored
// WorkflowState field is the single source of truth; a task that carries no
// stored state (only possible for a zero-value in-memory task, since the loader
// always derives one) resolves to StateReady.
func (t *Task) State() WorkflowState {
	if t.WorkflowState != "" {
		return t.WorkflowState
	}
	return StateReady
}

// IsInState reports whether the task's state equals the given state.
func (t *Task) IsInState(state WorkflowState) bool {
	return t.State() == state
}

// ApplyWorkflowState stores the canonical state. It is a plain assignment: the
// legacy Status/Section/Stage fields have been retired, so there is no longer
// any backing field to keep coherent. CompletedAt is intentionally left
// untouched: the terminal timestamp is carried by the independent `completed`
// metadata field, not derived here.
func (t *Task) ApplyWorkflowState(state WorkflowState) {
	t.WorkflowState = state
}

// SectionForState maps a workflow state to the legacy GTD section bucket that
// state occupies in section-oriented views (Kanban columns, section counts,
// GetTasksBySection). It is the display-bucket companion to ApplyWorkflowState:
// unlike the canonical-representative table there (which maps canceled to an
// empty section), both terminal states bucket under Completed because that is
// where finished work has always been listed. Ready and in-progress work lives
// in the default active bucket (empty string). Removed with the legacy section
// concept in session 33.7.
func SectionForState(s WorkflowState) string {
	switch s {
	case StateInbox:
		return SectionInbox
	case StateNext:
		return SectionNext
	case StateWaiting:
		return SectionWaiting
	case StatePaused:
		return SectionStandby
	case StateReview:
		return SectionReview
	case StateSomeday:
		return SectionSomeday
	case StateDone, StateCanceled:
		return SectionCompleted
	default: // StateReady, StateInProgress
		return ""
	}
}

// TargetStateForStatus maps a legacy target status to the workflow state that
// applying it realizes. It mirrors ApplyTransition's routing: a todo target
// invokes Reopen, which lands in the Inbox, so StatusTodo maps to StateInbox
// (not to the ready/next states a todo status can also derive from). Used by
// CanTransitionTo to validate legacy status transitions through the canonical
// state rules; removed with the legacy status API in session 33.7.
func TargetStateForStatus(status TaskStatus) WorkflowState {
	switch status {
	case StatusTodo:
		return StateInbox
	case StatusInProgress:
		return StateInProgress
	case StatusReview:
		return StateReview
	case StatusStandby:
		return StatePaused
	case StatusMaybe:
		return StateSomeday
	case StatusBlocked:
		return StateWaiting
	case StatusDone:
		return StateDone
	case StatusCanceled:
		return StateCanceled
	default:
		return WorkflowState(status) // invalid status -> invalid state, rejected by IsValid
	}
}

// LegacyStatusForState maps a workflow state back onto the legacy TaskStatus
// vocabulary. It is the inverse companion to TargetStateForStatus, collapsing
// the three todo-ish planning states (inbox/ready/next) onto StatusTodo,
// waiting onto blocked, paused onto standby, and someday onto maybe — mirroring
// the state collapse. Display surfaces (CLI JSON output, the workflow dry-run
// engine bridge) that must still emit or feed the legacy status string derive
// it from State() through this helper instead of reading the retired
// Task.Status field. Removed with the legacy status API in session 33.7.
func LegacyStatusForState(s WorkflowState) TaskStatus {
	switch s {
	case StateInProgress:
		return StatusInProgress
	case StateWaiting:
		return StatusBlocked
	case StatePaused:
		return StatusStandby
	case StateReview:
		return StatusReview
	case StateSomeday:
		return StatusMaybe
	case StateDone:
		return StatusDone
	case StateCanceled:
		return StatusCanceled
	default: // StateInbox, StateReady, StateNext
		return StatusTodo
	}
}

// IsTerminalState reports whether the task is in a terminal state (done or
// canceled). Prefer this over Status.IsComplete() at new call sites so the
// terminal test flows through the canonical state model.
func (t *Task) IsTerminalState() bool {
	return t.State().IsTerminal()
}

// IsDependencyBlocked reports whether the task is gated by an unresolved
// dependency within the given project. Dependency blocking is derived, not a
// workflow state: it is surfaced as a badge/filter and combines with the task's
// state rather than replacing it. A nil project yields false.
func (t *Task) IsDependencyBlocked(p *Project) bool {
	if p == nil {
		return false
	}
	return p.HasIncompleteDependencies(t)
}
