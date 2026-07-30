package models

import (
	"fmt"

	"github.com/sglavoie/tt/internal/icons"
)

// WorkflowState is the single canonical lifecycle state of a task.
type WorkflowState string

const (
	// StateInbox is unprocessed, freshly captured work awaiting triage.
	StateInbox WorkflowState = "inbox"
	// StateReady is triaged work with no scheduling commitment yet.
	StateReady WorkflowState = "ready"
	// StateNext is work explicitly queued as the next thing to do.
	StateNext WorkflowState = "next"
	// StateInProgress is work actively being executed.
	StateInProgress WorkflowState = "in_progress"
	// StateWaiting is work suspended on an external dependency ("waiting for").
	StateWaiting WorkflowState = "waiting"
	// StatePaused is work voluntarily suspended by the user.
	StatePaused WorkflowState = "paused"
	// StateSomeday is uncommitted, someday/maybe work.
	StateSomeday WorkflowState = "someday"
	// StateDone is successfully completed, terminal work.
	StateDone WorkflowState = "done"
	// StateCanceled is abandoned, terminal work.
	StateCanceled WorkflowState = "canceled"
)

// AllWorkflowStates returns all nine target workflow states in canonical order
// (nonterminal states first, then the two terminal states).
func AllWorkflowStates() []WorkflowState {
	return []WorkflowState{
		StateInbox,
		StateReady,
		StateNext,
		StateInProgress,
		StateWaiting,
		StatePaused,
		StateSomeday,
		StateDone,
		StateCanceled,
	}
}

// IsValid reports whether the state is one of the nine target states.
// Retired values accepted by migration readers are deliberately excluded.
func (s WorkflowState) IsValid() bool {
	switch s {
	case StateInbox, StateReady, StateNext, StateInProgress, StateWaiting,
		StatePaused, StateSomeday, StateDone, StateCanceled:
		return true
	case StateReview:
		return false
	}
	return false
}

// IsTerminal reports whether the state is terminal (done or canceled). Terminal
// tasks carry a CompletedAt timestamp and can only leave their state via reopen.
func (s WorkflowState) IsTerminal() bool {
	return s == StateDone || s == StateCanceled
}

// IsNonterminal reports whether the state is a valid, non-terminal state.
func (s WorkflowState) IsNonterminal() bool {
	return s.IsValid() && !s.IsTerminal()
}

// String returns the canonical string form of the state.
func (s WorkflowState) String() string {
	return string(s)
}

// Icon returns a one-character indicator for the state.
func (s WorkflowState) Icon() string {
	switch s {
	case StateInbox, StateReady, StateNext:
		return icons.Get("status.todo")
	case StateInProgress:
		return icons.Get("status.in_progress")
	case StateWaiting:
		return icons.Get("status.blocked")
	case StatePaused:
		return icons.Get("status.standby")
	case StateSomeday:
		return icons.Get("status.maybe")
	case StateDone:
		return icons.Get("status.done")
	case StateCanceled:
		return icons.Get("status.canceled")
	default:
		return "?"
	}
}

// ParseWorkflowState converts a string to a WorkflowState, reporting whether it
// names a valid canonical state.
func ParseWorkflowState(str string) (WorkflowState, bool) {
	s := WorkflowState(str)
	return s, s.IsValid()
}

// ReopenTargetState is the state a task always returns to when reopened from a
// terminal state. Reopen is deliberately not configurable: a completed or
// canceled task re-enters the workflow at the Inbox for re-triage.
const ReopenTargetState = StateInbox

// CanTransitionState reports whether a task may move directly from state `from`
// to state `to`, returning an empty string when the transition is allowed or a
// human-readable reason when it is not.
//
// Rules (see context.md "Task model"):
//   - Any nonterminal state may transition to any other valid state, including
//     the terminal done/canceled states (complete / cancel).
//   - Terminal states (done, canceled) may only transition to StateInbox, which
//     is the reopen path; every other target requires reopening first.
func CanTransitionState(from, to WorkflowState) string {
	if !from.IsValid() {
		return fmt.Sprintf("invalid current state %q", from)
	}
	if !to.IsValid() {
		return fmt.Sprintf("invalid target state %q", to)
	}
	if from == to {
		return fmt.Sprintf("task is already in state %q", from)
	}
	if from.IsTerminal() && to != ReopenTargetState {
		return fmt.Sprintf("cannot move task from terminal state %q to %q (reopen it first)", from, to)
	}
	return ""
}
