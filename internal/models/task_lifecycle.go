package models

import "time"

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
	// Auto-triage: move out of Inbox/Next/Waiting/Someday/Review when starting
	if t.Section == SectionInbox || t.Section == SectionNext || t.Section == SectionWaiting || t.Section == SectionSomeday || t.Section == SectionReview {
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

// Block marks the task as blocked.
func (t *Task) Block() {
	t.Status = StatusBlocked
	t.UpdatedAt = time.Now()
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
		return "cannot change status of completed task"
	}
	switch newStatus {
	case StatusInProgress:
		if t.Status != StatusTodo && t.Status != StatusBlocked && t.Status != StatusReview {
			return "task must be todo, blocked, or in review to start"
		}
	case StatusDone:
		if !t.Status.IsActive() {
			return "task is not active"
		}
	case StatusBlocked:
		if !t.Status.IsActive() || t.Status == StatusBlocked {
			return "task must be active and not already blocked"
		}
	case StatusCanceled:
		if !t.Status.IsActive() {
			return "task is not active"
		}
	case StatusTodo:
		if t.Status == StatusTodo {
			return "task is already todo"
		}
	case StatusReview:
		if !t.Status.IsActive() || t.Status == StatusReview {
			return "task must be active and not already in review"
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
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	snoozeDay := time.Date(t.SnoozedUntil.Year(), t.SnoozedUntil.Month(), t.SnoozedUntil.Day(), 0, 0, 0, 0, now.Location())
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
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	dueDay := time.Date(t.DueDate.Year(), t.DueDate.Month(), t.DueDate.Day(), 0, 0, 0, 0, now.Location())
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
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	dueDay := time.Date(t.DueDate.Year(), t.DueDate.Month(), t.DueDate.Day(), 0, 0, 0, 0, now.Location())
	// Not overdue
	if dueDay.Before(today) {
		return false
	}
	// Within N days (including today)
	threshold := today.AddDate(0, 0, days)
	return !dueDay.After(threshold)
}

// SetRecurrence sets the recurrence pattern for the task.
func (t *Task) SetRecurrence(pattern *RecurrencePattern) {
	t.Recurrence = pattern
	t.UpdatedAt = time.Now()
}

// ClearRecurrence removes the recurrence pattern from the task.
func (t *Task) ClearRecurrence() {
	if t.Recurrence != nil {
		t.Recurrence = nil
		t.UpdatedAt = time.Now()
	}
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
