package models

import (
	"fmt"
	"time"

	"github.com/sglavoie/tt/internal/util"
)

// Complete marks the task as done and sets the completion time.
// The task is moved to the Completed section.
func (t *Task) Complete() {
	now := time.Now()
	t.Status = StatusDone
	t.CompletedAt = &now
	t.UpdatedAt = now
	t.Active = false
	t.Section = SectionCompleted
}

// Start marks the task as in progress.
// If the task is in any non-Active section, it is automatically moved out (auto-triage).
func (t *Task) Start() {
	t.Status = StatusInProgress
	t.UpdatedAt = time.Now()
	// Auto-triage: move out of Inbox/Next/Waiting/Someday/Review/Standby when starting
	if t.Section == SectionInbox || t.Section == SectionNext || t.Section == SectionWaiting || t.Section == SectionSomeday || t.Section == SectionReview || t.Section == SectionStandby {
		t.Section = ""
	}
}

// Review marks the task as in review.
// If the task is in any non-Active section, it is automatically moved to Review section.
func (t *Task) Review() {
	t.Status = StatusReview
	t.UpdatedAt = time.Now()
	t.Section = SectionReview
}

// Standby puts the task on hold.
// The task is moved to the Standby section.
func (t *Task) Standby() {
	t.Status = StatusStandby
	t.UpdatedAt = time.Now()
	t.Section = SectionStandby
}

// Block marks the task as blocked.
func (t *Task) Block() {
	t.Status = StatusBlocked
	t.UpdatedAt = time.Now()
	t.Section = SectionWaiting
}

// Cancel marks the task as canceled.
// The task is moved to the Completed section.
func (t *Task) Cancel() {
	now := time.Now()
	t.Status = StatusCanceled
	t.CompletedAt = &now
	t.UpdatedAt = now
	t.Section = SectionCompleted
}

// Reopen sets the task back to todo status and moves it to the Inbox for re-processing.
func (t *Task) Reopen() {
	t.Status = StatusTodo
	t.CompletedAt = nil
	t.Active = false
	t.Section = SectionInbox
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
		if t.Status != StatusTodo && t.Status != StatusBlocked && t.Status != StatusReview && t.Status != StatusStandby {
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

// IsSnoozed returns true if the task is snoozed (SnoozedUntil is set and after today).
func (t *Task) IsSnoozed() bool {
	if t.SnoozedUntil == nil {
		return false
	}
	now := time.Now()
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
	today := util.StartOfDay(now)
	dueDay := util.StartOfDay(*t.DueDate)
	return dueDay.Before(today)
}

// IsDueSoon returns true if the task has a due date within the specified number
// of days (inclusive) and is not overdue or complete.
// Uses day-level comparison (normalizes to midnight local time).
func (t *Task) IsDueSoon(days int) bool {
	if t.DueDate == nil || t.Status.IsComplete() {
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

// SetRecurrence sets the recurrence pattern for the task.
// Returns an error if the pattern is non-nil and fails validation.
// For monthly patterns, OriginalDay is automatically populated from the task's
// DueDate (if set) to prevent day-of-month drift across short months.
func (t *Task) SetRecurrence(pattern *RecurrencePattern) error {
	if pattern != nil {
		if err := pattern.Validate(); err != nil {
			return err
		}
		// Populate OriginalDay for monthly patterns so that months with fewer
		// days (e.g., Feb 28) don't permanently drift the day-of-month.
		if pattern.Unit == RecurrenceMonthly && pattern.OriginalDay == 0 && t.DueDate != nil {
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

// SkipRecurrence advances the due date to the next occurrence without completing
// the task. Increments the Skipped counter and resets the Streak to 0 (skipping
// breaks a streak). No-op if the task is not recurring.
func (t *Task) SkipRecurrence() {
	if t.Recurrence == nil {
		return
	}

	next := t.NextDueDate()
	if next != nil {
		t.DueDate = next
	}
	t.Recurrence.Skipped++
	t.Recurrence.Streak = 0
	t.UpdatedAt = time.Now()
}

// DeferRecurrence pushes the due date forward by the given number of days.
// Unlike SkipRecurrence, deferring does not affect the streak or skipped count.
// No-op if the task is not recurring.
func (t *Task) DeferRecurrence(days int) {
	if t.Recurrence == nil {
		return
	}

	base := util.StartOfDay(time.Now())
	if t.DueDate != nil {
		base = util.StartOfDay(*t.DueDate)
	}
	newDue := base.AddDate(0, 0, days)
	t.DueDate = &newDue
	t.UpdatedAt = time.Now()
}

// NextDueDate calculates the next due date based on the recurrence pattern.
// Uses fixed interval scheduling: next date is calculated from the current due date.
// If the task has no due date, calculates from today.
// If the task is not recurring, returns nil.
func (t *Task) NextDueDate() *time.Time {
	if t.Recurrence == nil {
		return nil
	}

	baseDate := time.Now()
	if t.DueDate != nil {
		baseDate = *t.DueDate
	}

	next := calculateNextDate(baseDate, t.Recurrence)
	return &next
}

// NextNDueDates returns the next n upcoming due dates based on the recurrence
// pattern. Each date is calculated by iterating from the previous one. If the
// task has no due date, calculations start from today. Returns nil if the task
// is not recurring.
func (t *Task) NextNDueDates(n int) []time.Time {
	if t.Recurrence == nil || n <= 0 {
		return nil
	}

	baseDate := time.Now()
	if t.DueDate != nil {
		baseDate = *t.DueDate
	}

	dates := make([]time.Time, 0, n)
	current := baseDate
	for range n {
		current = calculateNextDate(current, t.Recurrence)
		dates = append(dates, current)
	}
	return dates
}
