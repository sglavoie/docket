package models

import (
	"fmt"
	"time"

	"github.com/sglavoie/tt/internal/util"
)

// Complete marks the task as done and sets the completion time.
func (t *Task) Complete() {
	_ = t.ApplyStateTransition(StateDone)
}

// Start marks the task as Doing.
// Deferral is cleared since a Doing task should always be visible.
func (t *Task) Start() {
	_ = t.ApplyStateTransition(StateDoing)
}

// Standby puts the task on hold (the paused state).
// Deferral is cleared since paused tasks should remain visible.
func (t *Task) Standby() {
	_ = t.ApplyStateTransition(StatePaused)
}

// Maybe marks the task as a tentative consideration (the someday state).
// Deferral is cleared since maybe tasks should remain visible for review.
func (t *Task) Maybe() {
	_ = t.ApplyStateTransition(StateSomeday)
}

// Wait marks the task as explicitly waiting.
// Deferral is cleared since waiting work should remain visible.
func (t *Task) Wait() {
	_ = t.ApplyStateTransition(StateWaiting)
}

// Cancel marks the task as canceled (terminal).
// Deferral is cleared since hiding a canceled task is meaningless.
func (t *Task) Cancel() {
	_ = t.ApplyStateTransition(StateCanceled)
}

// Uncomplete reverts a completed task to Doing without relocating it to
// Inbox, so it stays visible in place.
func (t *Task) Uncomplete() {
	_ = t.ApplyStateTransition(StateDoing)
}

// ApplyStateTransition moves the task to the given canonical WorkflowState,
// applying all lifecycle side effects at time.Now. The caller should check
// CanTransitionState first when transition policy must be enforced.
func (t *Task) ApplyStateTransition(state WorkflowState) error {
	return t.ApplyStateTransitionAt(state, time.Now())
}

// ApplyStateTransitionAt is the canonical task-state mutation primitive. It
// owns terminal completion timestamps, reopen cleanup, deferral clearing, the
// state clock, and the update timestamp. Supplying now lets application
// services use their injected clock.
//
// This method validates the target but deliberately does not enforce
// CanTransitionState: reconciliation may perform explicit overrides.
// User-facing transition services validate policy first.
func (t *Task) ApplyStateTransitionAt(state WorkflowState, now time.Time) error {
	if !state.IsValid() {
		return fmt.Errorf("invalid workflow state %q", state)
	}

	// Re-applying the state a task already holds is not a transition, so the
	// state clock must survive it: `ww` on a task that has waited twelve days
	// still reads twelve days.
	if t.State() != state {
		t.StateChangedAt = now
	}
	t.ApplyWorkflowState(state)
	t.DeferredUntil = nil
	if state.IsTerminal() {
		completedAt := now
		t.CompletedAt = &completedAt
		// A finished task owes no further look, so completion and cancellation
		// clear the check-in date and its repeat. Review history is not owed
		// work and survives. Because the schedule is dropped here rather than on
		// the way back out, reopening inherits nothing: an old cadence silently
		// resuming months later is a reminder nobody asked for.
		t.clearCheckInForTerminal()
	} else {
		t.CompletedAt = nil
	}
	t.UpdatedAt = now
	return nil
}

// IsComplete returns true if the task is in a terminal workflow state (done or
// canceled). It is a synonym for IsTerminalState, kept for the large existing
// caller base.
func (t *Task) IsComplete() bool {
	return t.State().IsTerminal()
}

// Pin marks the task as pinned.
func (t *Task) Pin() {
	now := time.Now()
	t.Pinned = true
	t.PinnedAt = &now
	t.UpdatedAt = now
}

// ApplyPinned moves the task to the requested pinned state and reports whether
// anything changed. Re-pinning an already pinned task preserves PinnedAt, which
// is what keeps the manual J/K pinned order stable across a re-declaring
// outline buffer or a repeated *pin edit.
func (t *Task) ApplyPinned(pinned bool, now time.Time) bool {
	if t.Pinned == pinned {
		return false
	}
	t.Pinned = pinned
	if pinned {
		pinnedAt := now
		t.PinnedAt = &pinnedAt
	} else {
		t.PinnedAt = nil
	}
	t.UpdatedAt = now
	return true
}

// ApplyReview moves the task to the requested review state and reports whether
// anything changed. Review carries no timestamp of its own: unlike a pin, whose
// instant orders the pinned section, nothing reads when a task was flagged, so
// storing one would be a field no reader could justify.
func (t *Task) ApplyReview(review bool, now time.Time) bool {
	if t.Review == review {
		return false
	}
	t.Review = review
	t.UpdatedAt = now
	return true
}

// AcknowledgeReview records that a task has been reviewed: the flag comes off,
// the review history is stamped, and the touch clock moves, unconditionally. It
// is not ApplyReview(false): clearing a flag that is already clear changes
// nothing, but acknowledging a scheduled, stale, or Waiting task that was never
// flagged is the whole point of the operation, and what makes it an
// acknowledgement is that UpdatedAt moves.
//
// A scheduled task also has its check-in answered here, from the day the review
// actually happened rather than from the date that was owed: a one-off check-in
// is cleared and a repeating one is moved forward its own cadence. That is the
// whole schedule effect, and it is computed before anything is written, so a
// cadence that cannot land on a storable date leaves the task exactly as it was
// instead of half-acknowledged.
//
// Nothing else is written. The lifecycle state and its clock, completion data,
// deadlines, deferral, planning membership, pin, and resume note all survive, so
// a Waiting task stays Waiting with its check-back date and its waiting age
// intact.
func (t *Task) AcknowledgeReview(now time.Time) error {
	next := t.NextCheckIn
	if next != nil {
		scheduled, err := NextCheckInAfter(now, t.CheckInRepeatDays)
		if err != nil {
			return err
		}
		next = scheduled
	}
	reviewedAt := now
	t.LastReviewedAt = &reviewedAt
	t.NextCheckIn = next
	if next == nil {
		t.CheckInRepeatDays = CheckInRepeatOff
	}
	t.Review = false
	t.UpdatedAt = now
	return nil
}

// Unpin removes the pinned status.
func (t *Task) Unpin() {
	t.Pinned = false
	t.PinnedAt = nil
	t.UpdatedAt = time.Now()
}

// TogglePinned toggles the pinned status.
func (t *Task) TogglePinned() {
	if t.Pinned {
		t.Unpin()
	} else {
		t.Pin()
	}
}

// IsDeferred returns true if the task is deferred (DeferredUntil is set and after now).
func (t *Task) IsDeferred() bool {
	return t.IsDeferredAt(time.Now())
}

// IsDeferredAt returns true if the task is deferred relative to the given time.
// Use this instead of IsDeferred when the reference time is not time.Now().
// DeferredUntil is an instant, not a calendar date, so the comparison is
// location-independent by construction and a deferral can expire mid-day.
func (t *Task) IsDeferredAt(now time.Time) bool {
	if t.DeferredUntil == nil {
		return false
	}
	return t.DeferredUntil.After(now)
}

// ApplyDeferAt is the canonical defer lifecycle transition. Deferring always
// claims Backlog, and only a new claim increments DeferCount: moving an
// unexpired wake instant is an edit to the pending deferral, while setting a
// new instant after the previous one lapsed is a repeat deferral. The passage
// of time itself never calls this method and therefore never changes the count.
func (t *Task) ApplyDeferAt(until, now time.Time) error {
	wasPending := t.IsDeferredAt(now)
	if err := t.ApplyStateTransitionAt(StateBacklog, now); err != nil {
		return err
	}
	deferredUntil := until
	t.DeferredUntil = &deferredUntil
	if !wasPending {
		t.DeferCount++
	}
	return nil
}

// DueHasTime reports whether the task's due date carries a meaningful
// time-of-day. A date-only due date is normalized to local midnight, so a
// non-midnight clock is the signal that the user set an explicit hh:mm time.
// The single practical consequence is that "due at exactly 00:00" is treated
// as a date-only due date; every other clock time is an instant-precise due.
func (t *Task) DueHasTime() bool {
	if t.DueDate == nil {
		return false
	}
	return !t.DueDate.Equal(util.StartOfDay(*t.DueDate))
}

// IsOverdue returns true if the task has a due date in the past and is not complete.
// A date-only due date (no explicit time) is compared at day granularity
// (normalized to midnight local time), so it is overdue only once the whole day
// has passed. A timed due date is compared at its exact instant, so a task due
// today at 09:00 is overdue from 10:00.
func (t *Task) IsOverdue() bool {
	if t.DueDate == nil || t.IsTerminalState() {
		return false
	}
	now := time.Now()
	if t.DueHasTime() {
		return t.DueDate.Before(now)
	}
	today := util.StartOfDay(now)
	dueDay := util.StartOfDay(*t.DueDate)
	return dueDay.Before(today)
}

// IsDueToday returns true if the task has a due date that falls on today.
// Does not check completion status — callers can combine with IsComplete() if needed.
// Uses day-level comparison (normalizes to midnight local time).
func (t *Task) IsDueToday() bool {
	if t.DueDate == nil {
		return false
	}
	now := time.Now()
	today := util.StartOfDay(now)
	dueDay := util.StartOfDay(*t.DueDate)
	return dueDay.Equal(today)
}
