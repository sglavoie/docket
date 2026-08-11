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
// Snooze is cleared since a Doing task should always be visible.
func (t *Task) Start() {
	_ = t.ApplyStateTransition(StateDoing)
}

// Standby puts the task on hold (the paused state).
// Snooze is cleared since paused tasks should remain visible.
func (t *Task) Standby() {
	_ = t.ApplyStateTransition(StatePaused)
}

// Maybe marks the task as a tentative consideration (the someday state).
// Snooze is cleared since maybe tasks should remain visible for review.
func (t *Task) Maybe() {
	_ = t.ApplyStateTransition(StateSomeday)
}

// Wait marks the task as explicitly waiting.
// Snooze is cleared since waiting work should remain visible.
func (t *Task) Wait() {
	_ = t.ApplyStateTransition(StateWaiting)
}

// Cancel marks the task as canceled (terminal).
// Snooze is cleared since hiding a canceled task is meaningless.
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
// owns terminal completion timestamps, reopen cleanup, snooze clearing, the
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
	// state clock must survive it: `sw` on a task that has waited twelve days
	// still reads twelve days.
	if t.State() != state {
		t.StateChangedAt = now
	}
	t.ApplyWorkflowState(state)
	t.SnoozedUntil = nil
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

// IsSnoozed returns true if the task is snoozed (SnoozedUntil is set and after today).
func (t *Task) IsSnoozed() bool {
	return t.IsSnoozedAt(time.Now())
}

// IsSnoozedAt returns true if the task is snoozed relative to the given time.
// Use this instead of IsSnoozed when the reference time is not time.Now().
func (t *Task) IsSnoozedAt(now time.Time) bool {
	if t.SnoozedUntil == nil {
		return false
	}
	// SnoozedUntil is a calendar date, not an instant. Normalize both dates
	// onto the same neutral location before comparing so equal local dates do
	// not compare differently merely because their time.Location values differ.
	nowYear, nowMonth, nowDay := now.Date()
	snoozeYear, snoozeMonth, snoozeDayOfMonth := t.SnoozedUntil.Date()
	today := time.Date(nowYear, nowMonth, nowDay, 0, 0, 0, 0, time.UTC)
	snoozeDay := time.Date(snoozeYear, snoozeMonth, snoozeDayOfMonth, 0, 0, 0, 0, time.UTC)
	return snoozeDay.After(today)
}

// ClearSnooze removes the snooze date from the task.
func (t *Task) ClearSnooze() {
	t.SnoozedUntil = nil
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
