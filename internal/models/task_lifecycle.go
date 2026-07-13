package models

import (
	"fmt"
	"time"

	"github.com/sglavoie/tt/internal/util"
)

// Complete marks the task as done and sets the completion time.
// Snooze and parking are cleared since hiding or resuming a completed task is meaningless.
func (t *Task) Complete() {
	now := time.Now()
	t.Unpark(true)
	t.WorkflowState = StateDone
	t.CompletedAt = &now
	t.UpdatedAt = now
	t.SnoozedUntil = nil
}

// Start marks the task as in progress.
// Snooze is cleared since an in-progress task should always be visible.
func (t *Task) Start() {
	t.WorkflowState = StateInProgress
	t.UpdatedAt = time.Now()
	t.SnoozedUntil = nil
}

// Review marks the task as in review.
// Snooze is cleared since a task in review should always be visible.
func (t *Task) Review() {
	t.WorkflowState = StateReview
	t.UpdatedAt = time.Now()
	t.SnoozedUntil = nil
}

// Standby puts the task on hold (the paused state).
// Snooze is cleared since paused tasks should remain visible.
func (t *Task) Standby() {
	t.WorkflowState = StatePaused
	t.UpdatedAt = time.Now()
	t.SnoozedUntil = nil
}

// Maybe marks the task as a tentative consideration (the someday state).
// Snooze is cleared since maybe tasks should remain visible for review.
func (t *Task) Maybe() {
	t.WorkflowState = StateSomeday
	t.UpdatedAt = time.Now()
	t.SnoozedUntil = nil
}

// Block marks the task as waiting (blocked on external input).
// Snooze is cleared since a waiting task needs visibility for tracking blockers.
func (t *Task) Block() {
	t.WorkflowState = StateWaiting
	t.UpdatedAt = time.Now()
	t.SnoozedUntil = nil
}

// Cancel marks the task as canceled (terminal).
// Snooze is cleared since hiding a canceled task is meaningless.
func (t *Task) Cancel() {
	now := time.Now()
	t.WorkflowState = StateCanceled
	t.CompletedAt = &now
	t.UpdatedAt = now
	t.SnoozedUntil = nil
}

// Reopen returns the task to the Inbox for re-processing.
// Snooze is cleared so the reopened task is immediately visible.
func (t *Task) Reopen() {
	t.WorkflowState = StateInbox
	t.CompletedAt = nil
	t.Dimmed = false
	t.SnoozedUntil = nil
	t.UpdatedAt = time.Now()
}

// Uncomplete reverts a completed task to in-progress without relocating it to
// Inbox, so it stays visible in place.
func (t *Task) Uncomplete() {
	t.WorkflowState = StateInProgress
	t.CompletedAt = nil
	t.UpdatedAt = time.Now()
}

// CanTransitionTo checks whether the task can move to newStatus. Returns an
// empty string if the transition is allowed, or a human-readable reason if it
// is not.
//
// This is the legacy-status wrapper over the canonical state rules: the target
// status is mapped to the workflow state applying it would realize (see
// TargetStateForStatus) and validated with CanTransitionState against the
// task's current State(). Nonterminal states are freely interchangeable, a
// same-state transition is rejected, and terminal states only allow the todo
// (reopen-to-inbox) target.
//
// This method covers state-level rules only. Project-level checks (such as
// incomplete dependencies blocking completion) must be performed by the caller.
func (t *Task) CanTransitionTo(newStatus TaskStatus) string {
	if !newStatus.IsValid() {
		return fmt.Sprintf("invalid target status %q", newStatus)
	}
	return CanTransitionState(t.State(), TargetStateForStatus(newStatus))
}

// ApplyTransition moves the task to newStatus by calling the appropriate
// lifecycle method (Start, Complete, Block, Cancel, Review, Reopen).
// The caller should check CanTransitionTo first.
func (t *Task) ApplyTransition(newStatus TaskStatus) {
	switch newStatus {
	case StatusInProgress:
		t.Start()
	case StatusDone:
		t.Complete()
	case StatusBlocked:
		t.Block()
	case StatusCanceled:
		t.Cancel()
	case StatusTodo:
		t.Reopen()
	case StatusReview:
		t.Review()
	case StatusStandby:
		t.Standby()
	case StatusMaybe:
		t.Maybe()
	}
}

// ApplyStateTransition moves the task to the given canonical WorkflowState,
// invoking the appropriate lifecycle method so terminal timestamps (CompletedAt),
// reopen-to-inbox, and snooze-clearing side effects stay consistent. It is the
// WorkflowState-native successor to ApplyTransition(TaskStatus): interactive and
// bulk transition controls call it directly with one of the ten states rather
// than routing a legacy TaskStatus through TargetStateForStatus first.
//
// The ready and next states have no dedicated legacy lifecycle mutator, so they
// are applied through ApplyWorkflowState (which sets the canonical backing
// fields) with an explicit snooze clear and UpdatedAt bump to match the other
// nonterminal transitions. The caller should check CanTransitionState first.
func (t *Task) ApplyStateTransition(state WorkflowState) {
	switch state {
	case StateInProgress:
		t.Start()
	case StateDone:
		t.Complete()
	case StateWaiting:
		t.Block()
	case StateCanceled:
		t.Cancel()
	case StateInbox:
		t.Reopen()
	case StateReview:
		t.Review()
	case StatePaused:
		t.Standby()
	case StateSomeday:
		t.Maybe()
	case StateReady, StateNext:
		t.SnoozedUntil = nil
		t.ApplyWorkflowState(state)
		t.UpdatedAt = time.Now()
	}
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

// Dim marks the task as visually dimmed.
func (t *Task) Dim() {
	t.Dimmed = true
	t.UpdatedAt = time.Now()
}

// Undim removes the dimmed state.
func (t *Task) Undim() {
	t.Dimmed = false
	t.UpdatedAt = time.Now()
}

// ToggleDimmed toggles the dimmed state.
func (t *Task) ToggleDimmed() {
	if t.Dimmed {
		t.Undim()
	} else {
		t.Dim()
	}
}

// IsSnoozed returns true if the task is snoozed (SnoozedUntil is set and after today).
func (t *Task) IsSnoozed() bool {
	return t.IsSnoozedAt(time.Now())
}

// IsSnoozedAt returns true if the task is snoozed relative to the given time.
// Use this instead of IsSnoozed when the reference time is not time.Now()
// (e.g., in alert evaluation with a deterministic clock).
func (t *Task) IsSnoozedAt(now time.Time) bool {
	if t.SnoozedUntil == nil {
		return false
	}
	today := util.StartOfDay(now)
	snoozeDay := util.StartOfDay(*t.SnoozedUntil)
	return snoozeDay.After(today)
}

// ClearSnooze removes the snooze date from the task.
func (t *Task) ClearSnooze() {
	t.SnoozedUntil = nil
	t.UpdatedAt = time.Now()
}

// IsRecurring returns true if the task has a recurrence pattern.
func (t *Task) IsRecurring() bool {
	return t.Recurrence != nil
}

// IsOverdue returns true if the task has a due date in the past and is not complete.
// Uses day-level comparison (normalizes to midnight local time).
func (t *Task) IsOverdue() bool {
	if t.DueDate == nil || t.IsTerminalState() {
		return false
	}
	now := time.Now()
	if t.HasTime {
		return t.DueDate.Before(now)
	}
	today := util.StartOfDay(now)
	dueDay := util.StartOfDay(*t.DueDate)
	return dueDay.Before(today)
}

// IsDueSoon returns true if the task has a due date within the specified number
// of days (inclusive) and is not overdue or complete.
// When HasTime is set, uses exact time comparison so that a task whose specific
// time has already passed is treated as overdue (not "due soon").
// Otherwise uses day-level comparison (normalizes to midnight local time).
func (t *Task) IsDueSoon(days int) bool {
	if t.DueDate == nil || t.IsTerminalState() {
		return false
	}
	now := time.Now()
	if t.HasTime {
		// Time-aware: overdue if the exact time has passed
		if t.DueDate.Before(now) {
			return false
		}
		threshold := now.AddDate(0, 0, days)
		return !t.DueDate.After(threshold)
	}
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

// SetRecurrence sets the recurrence pattern for the task.
// Returns an error if the pattern is non-nil and fails validation.
// For monthly and yearly patterns, OriginalDay is automatically populated from
// the task's DueDate (if set) to prevent day-of-month drift across short months
// (monthly: e.g. Jan 31 → Feb 28 → Mar 31; yearly: e.g. Feb 29 → Feb 28 → Feb 29).
func (t *Task) SetRecurrence(pattern *RecurrencePattern) error {
	if pattern != nil {
		if err := pattern.Validate(); err != nil {
			return err
		}
		// Populate OriginalDay for monthly/yearly patterns so that months with
		// fewer days don't permanently drift the day-of-month.
		needsOriginalDay := pattern.Unit == RecurrenceMonthly || pattern.Unit == RecurrenceYearly
		if needsOriginalDay && pattern.OriginalDay == 0 && t.DueDate != nil {
			pattern.OriginalDay = t.DueDate.Day()
		}
	}
	t.Recurrence = pattern
	t.UpdatedAt = time.Now()
	return nil
}

// ClearRecurrence removes the recurrence pattern from the task.
func (t *Task) ClearRecurrence() {
	if t.Recurrence != nil {
		t.Recurrence = nil
		t.UpdatedAt = time.Now()
	}
}

// ensureRecurrenceOriginalDay populates Recurrence.OriginalDay for monthly and
// yearly patterns when it has not been set yet, using the given seed day. This
// guards against day-of-month drift when SetRecurrence ran before DueDate was
// assigned (so SetRecurrence could not seed OriginalDay itself): without this,
// the first short-month clamp would silently overwrite the user's intended day
// (e.g. Jan 31 → Feb 29 → Mar 29 instead of Mar 31).
func (t *Task) ensureRecurrenceOriginalDay(seedDay int) {
	if t.Recurrence == nil || t.Recurrence.OriginalDay != 0 {
		return
	}
	if t.Recurrence.Unit != RecurrenceMonthly && t.Recurrence.Unit != RecurrenceYearly {
		return
	}
	if seedDay >= 1 && seedDay <= 31 {
		t.Recurrence.OriginalDay = seedDay
	}
}

// SkipRecurrence advances the due date to the next occurrence without completing
// the task. Increments the Skipped counter and resets the Streak to 0 (skipping
// breaks a streak). No-op if the task is not recurring.
//
// The next date is always computed from the current due date (or today when no
// due date is set), regardless of the recurrence mode. The mode only governs
// post-completion scheduling; skipping is schedule-based and should not be
// influenced by when the task was (or will be) completed.
func (t *Task) SkipRecurrence() {
	if t.Recurrence == nil {
		return
	}

	var base time.Time
	if t.DueDate != nil {
		base = *t.DueDate
	} else if t.HasTime {
		base = time.Now()
	} else {
		base = util.StartOfDay(time.Now())
	}
	t.ensureRecurrenceOriginalDay(base.Day())
	next := calculateNextDate(base, t.Recurrence)
	t.DueDate = &next
	t.Recurrence.Skipped++
	t.Recurrence.Streak = 0
	t.UpdatedAt = time.Now()
}

// DeferRecurrence pushes the due date forward by the given number of days.
// Unlike SkipRecurrence, deferring does not affect the streak or skipped count.
// No-op if the task is not recurring or if days <= 0 (deferral must move the
// due date forward; a negative or zero value would silently rewind it).
// When the task carries a time-of-day (HasTime), the clock component is preserved.
func (t *Task) DeferRecurrence(days int) {
	if t.Recurrence == nil {
		return
	}
	if days <= 0 {
		return
	}

	if t.DueDate != nil && t.HasTime {
		// Preserve the clock component by adding days directly.
		newDue := t.DueDate.AddDate(0, 0, days)
		t.DueDate = &newDue
	} else {
		base := util.StartOfDay(time.Now())
		if t.DueDate != nil {
			base = util.StartOfDay(*t.DueDate)
		}
		newDue := base.AddDate(0, 0, days)
		t.DueDate = &newDue
	}
	t.UpdatedAt = time.Now()
}

// NextDueDate calculates the next due date based on the recurrence pattern.
// For RecurModeFromDue (default): next date is calculated from the current due date.
// For RecurModeFromCompletion: next date is calculated from CompletedAt (or now if nil).
// If the task has no due date (FromDue mode), calculates from today.
// If the task is not recurring, returns nil.
func (t *Task) NextDueDate() *time.Time {
	if t.Recurrence == nil {
		return nil
	}

	var baseDate time.Time
	if t.Recurrence.Mode == RecurModeFromCompletion {
		if t.CompletedAt != nil {
			baseDate = *t.CompletedAt
		} else {
			baseDate = time.Now()
		}
	} else {
		baseDate = time.Now()
		if t.DueDate != nil {
			baseDate = *t.DueDate
		}
	}

	// Seed OriginalDay on the first calculation when SetRecurrence ran before
	// DueDate was set; otherwise day-of-month drifts on short-month clamps.
	if t.Recurrence.Mode != RecurModeFromCompletion && t.DueDate != nil {
		t.ensureRecurrenceOriginalDay(t.DueDate.Day())
	}
	next := calculateNextDate(baseDate, t.Recurrence)
	return &next
}

// NextNDueDates returns the next n upcoming due dates based on the recurrence
// pattern. The first date matches NextDueDate (honoring Mode); subsequent
// dates iterate forward from there. Returns nil if the task is not recurring
// or n <= 0.
func (t *Task) NextNDueDates(n int) []time.Time {
	if t.Recurrence == nil || n <= 0 {
		return nil
	}

	var baseDate time.Time
	if t.Recurrence.Mode == RecurModeFromCompletion {
		if t.CompletedAt != nil {
			baseDate = *t.CompletedAt
		} else {
			baseDate = time.Now()
		}
	} else {
		baseDate = time.Now()
		if t.DueDate != nil {
			baseDate = *t.DueDate
		}
	}

	// Seed OriginalDay so that the iterated series does not drift on short months.
	if t.Recurrence.Mode != RecurModeFromCompletion && t.DueDate != nil {
		t.ensureRecurrenceOriginalDay(t.DueDate.Day())
	}
	dates := make([]time.Time, 0, n)
	current := baseDate
	for range n {
		current = calculateNextDate(current, t.Recurrence)
		dates = append(dates, current)
	}
	return dates
}
