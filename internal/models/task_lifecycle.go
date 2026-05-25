package models

import (
	"fmt"
	"time"

	"github.com/sglavoie/tt/internal/util"
)

// Complete marks the task as done and sets the completion time.
// The task is moved to the Completed section.
// Snooze is cleared since hiding a completed task is meaningless.
func (t *Task) Complete() {
	now := time.Now()
	t.Status = StatusDone
	t.CompletedAt = &now
	t.UpdatedAt = now
	t.Active = false
	t.SnoozedUntil = nil
	t.Section = SectionCompleted
	t.CustomSection = ""
}

// Start marks the task as in progress.
// If the task is in any non-Active section, it is automatically moved out (auto-triage).
// Snooze is cleared since an in-progress task should always be visible.
func (t *Task) Start() {
	t.Status = StatusInProgress
	t.UpdatedAt = time.Now()
	t.SnoozedUntil = nil
	// Auto-triage: move out of GTD holding sections when starting.
	// Also clear CustomSection so the task doesn't appear in two places.
	if t.Section == SectionInbox || t.Section == SectionNext || t.Section == SectionWaiting || t.Section == SectionSomeday || t.Section == SectionReview || t.Section == SectionStandby || t.Section == SectionCompleted {
		t.Section = ""
		t.CustomSection = ""
	}
}

// Review marks the task as in review.
// If the task is in any non-Active section, it is automatically moved to Review section.
// Snooze is cleared since a task in review should always be visible.
func (t *Task) Review() {
	t.Status = StatusReview
	t.UpdatedAt = time.Now()
	t.SnoozedUntil = nil
	t.Section = SectionReview
	t.CustomSection = ""
}

// Standby puts the task on hold.
// The task is moved to the Standby section.
// Snooze is cleared since standby tasks appear in the Standby section and should be visible.
func (t *Task) Standby() {
	t.Status = StatusStandby
	t.UpdatedAt = time.Now()
	t.SnoozedUntil = nil
	t.Section = SectionStandby
	t.CustomSection = ""
}

// Maybe marks the task as a tentative consideration.
// The task is moved to the Someday section.
// Snooze is cleared since maybe tasks should remain visible for review.
func (t *Task) Maybe() {
	t.Status = StatusMaybe
	t.UpdatedAt = time.Now()
	t.SnoozedUntil = nil
	t.Section = SectionSomeday
	t.CustomSection = ""
}

// Block marks the task as blocked.
// Snooze is cleared since a blocked task needs visibility for tracking blockers.
func (t *Task) Block() {
	t.Status = StatusBlocked
	t.UpdatedAt = time.Now()
	t.SnoozedUntil = nil
	t.Section = SectionWaiting
	t.CustomSection = ""
}

// Cancel marks the task as canceled.
// The task is moved to the Completed section.
// Snooze is cleared since hiding a canceled task is meaningless.
func (t *Task) Cancel() {
	now := time.Now()
	t.Status = StatusCanceled
	t.CompletedAt = &now
	t.UpdatedAt = now
	t.Active = false
	t.SnoozedUntil = nil
	t.Section = SectionCompleted
	t.CustomSection = ""
}

// Reopen sets the task back to todo status and moves it to the Inbox for re-processing.
// Snooze is cleared so the reopened task is immediately visible.
func (t *Task) Reopen() {
	t.Status = StatusTodo
	t.CompletedAt = nil
	t.Active = false
	t.Dimmed = false
	t.SnoozedUntil = nil
	t.Section = SectionInbox
	t.CustomSection = ""
	t.UpdatedAt = time.Now()
}

// CanTransitionTo checks whether the task can move to newStatus based on its
// current status. Returns an empty string if the transition is allowed, or a
// human-readable reason if it is not.
//
// This method covers status-level rules only. Project-level checks (such as
// incomplete dependencies blocking completion) must be performed by the caller.
func (t *Task) CanTransitionTo(newStatus TaskStatus) string {
	if t.Status.IsComplete() && newStatus != StatusTodo {
		return fmt.Sprintf("cannot change status of completed task (reopen it first with 'todo'), current status: '%s'", t.Status)
	}
	switch newStatus {
	case StatusInProgress:
		if t.Status != StatusTodo && t.Status != StatusBlocked && t.Status != StatusReview && t.Status != StatusStandby && t.Status != StatusMaybe {
			return fmt.Sprintf("cannot start task from '%s' status", t.Status)
		}
	case StatusDone:
		if !t.Status.IsActive() {
			return fmt.Sprintf("cannot complete task from '%s' status", t.Status)
		}
	case StatusBlocked:
		if !t.Status.IsActive() || t.Status == StatusBlocked {
			return fmt.Sprintf("cannot block task from '%s' status", t.Status)
		}
	case StatusCanceled:
		if !t.Status.IsActive() {
			return fmt.Sprintf("cannot cancel task from '%s' status", t.Status)
		}
	case StatusTodo:
		if t.Status == StatusTodo {
			return "task is already todo"
		}
	case StatusReview:
		if !t.Status.IsActive() || t.Status == StatusReview {
			return fmt.Sprintf("cannot move task to review from '%s' status", t.Status)
		}
	case StatusStandby:
		if !t.Status.IsActive() || t.Status == StatusStandby {
			return fmt.Sprintf("cannot put task on standby from '%s' status", t.Status)
		}
	case StatusMaybe:
		if !t.Status.IsActive() || t.Status == StatusMaybe {
			return fmt.Sprintf("cannot set task to maybe from '%s' status", t.Status)
		}
	}
	return ""
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

// IsComplete returns true if the task status is complete.
func (t *Task) IsComplete() bool {
	return t.Status.IsComplete()
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

// Activate marks the task as active.
func (t *Task) Activate() {
	t.Active = true
	t.Dimmed = false
	t.UpdatedAt = time.Now()
}

// Deactivate removes the active status.
func (t *Task) Deactivate() {
	t.Active = false
	t.UpdatedAt = time.Now()
}

// ToggleActive toggles the active status.
func (t *Task) ToggleActive() {
	if t.Active {
		t.Deactivate()
	} else {
		t.Activate()
	}
}

// Dim marks the task as visually dimmed and clears the Active flag.
func (t *Task) Dim() {
	t.Dimmed = true
	t.Active = false
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
	if t.DueDate == nil || t.Status.IsComplete() {
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
	if t.DueDate == nil || t.Status.IsComplete() {
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
