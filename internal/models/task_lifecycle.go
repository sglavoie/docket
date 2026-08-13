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

// IsDeferred returns true if the task is deferred (DeferredUntil is set and after today).
func (t *Task) IsDeferred() bool {
	return t.IsDeferredAt(time.Now())
}

// IsDeferredAt returns true if the task is deferred relative to the given time.
// Use this instead of IsDeferred when the reference time is not time.Now().
func (t *Task) IsDeferredAt(now time.Time) bool {
	if t.DeferredUntil == nil {
		return false
	}
	// DeferredUntil is a calendar date, not an instant. Normalize both dates
	// onto the same neutral location before comparing so equal local dates do
	// not compare differently merely because their time.Location values differ.
	nowYear, nowMonth, nowDay := now.Date()
	deferYear, deferMonth, deferDayOfMonth := t.DeferredUntil.Date()
	today := time.Date(nowYear, nowMonth, nowDay, 0, 0, 0, 0, time.UTC)
	deferDay := time.Date(deferYear, deferMonth, deferDayOfMonth, 0, 0, 0, 0, time.UTC)
	return deferDay.After(today)
}

// ClearDefer removes the deferral date from the task.
func (t *Task) ClearDefer() {
	t.DeferredUntil = nil
	t.UpdatedAt = time.Now()
}

// IsOverdue returns true if the task has a due date in the past and is not complete.
// Uses day-level comparison (normalizes to midnight local time).
func (t *Task) IsOverdue() bool {
	if t.DueDate == nil || t.IsTerminalState() {
		return false
	}
	now := time.Now()
	today := util.StartOfDay(now)
	dueDay := util.StartOfDay(*t.DueDate)
	return dueDay.Before(today)
}

// IsDueSoon returns true if the task has a due date within the specified number
// of days (inclusive) and is not overdue or complete.
// Uses day-level comparison (normalizes to midnight local time).
func (t *Task) IsDueSoon(days int) bool {
	if t.DueDate == nil || t.IsTerminalState() {
		return false
	}
	now := time.Now()
	today := util.StartOfDay(now)
	dueDay := util.StartOfDay(*t.DueDate)
	// Not overdue
	if dueDay.Before(today) {
		return false
	}
	// Within N days (including today)
	threshold := today.AddDate(0, 0, days)
	return !dueDay.After(threshold)
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

// IsDueThisWeek returns true if the task has a due date within the next 7 days
// (including today, excluding exactly 7 days from now).
// Does not check completion status — callers can combine with IsComplete() if needed.
// Uses day-level comparison (normalizes to midnight local time).
func (t *Task) IsDueThisWeek() bool {
	if t.DueDate == nil {
		return false
	}
	now := time.Now()
	today := util.StartOfDay(now)
	weekEnd := today.AddDate(0, 0, 7)
	dueDay := util.StartOfDay(*t.DueDate)
	return !dueDay.Before(today) && dueDay.Before(weekEnd)
}
