package models

import "time"

// Complete marks the task as done and sets the completion time.
// If the task is in a non-Active section, it is automatically moved out (auto-triage).
func (t *Task) Complete() {
	now := time.Now()
	t.Status = StatusDone
	t.CompletedAt = &now
	t.UpdatedAt = now
	t.Active = false
	// Auto-triage: move out of Next/Waiting/Someday when completing
	if t.Section == SectionNext || t.Section == SectionWaiting || t.Section == SectionSomeday {
		t.Section = ""
	}
}

// Start marks the task as in progress.
// If the task is in any non-Active section, it is automatically moved out (auto-triage).
func (t *Task) Start() {
	t.Status = StatusInProgress
	t.UpdatedAt = time.Now()
	// Auto-triage: move out of Inbox/Next/Waiting/Someday when starting
	if t.Section == SectionInbox || t.Section == SectionNext || t.Section == SectionWaiting || t.Section == SectionSomeday {
		t.Section = ""
	}
}

// Block marks the task as blocked.
func (t *Task) Block() {
	t.Status = StatusBlocked
	t.UpdatedAt = time.Now()
}

// Cancel marks the task as cancelled.
// If the task is in a non-Active section, it is automatically moved out (auto-triage).
func (t *Task) Cancel() {
	now := time.Now()
	t.Status = StatusCancelled
	t.CompletedAt = &now
	t.UpdatedAt = now
	// Auto-triage: move out of Next/Waiting/Someday when cancelling
	if t.Section == SectionNext || t.Section == SectionWaiting || t.Section == SectionSomeday {
		t.Section = ""
	}
}

// Reopen sets the task back to todo status and moves it to the Inbox for re-processing.
func (t *Task) Reopen() {
	t.Status = StatusTodo
	t.CompletedAt = nil
	t.Section = SectionInbox
	t.UpdatedAt = time.Now()
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

// IsRecurring returns true if the task has a recurrence pattern.
func (t *Task) IsRecurring() bool {
	return t.Recurrence != nil
}

// IsOverdue returns true if the task has a due date in the past and is not complete.
// Uses day-level comparison (normalizes to midnight UTC).
func (t *Task) IsOverdue() bool {
	if t.DueDate == nil || t.Status.IsComplete() {
		return false
	}
	today := time.Now().UTC().Truncate(24 * time.Hour)
	dueDay := t.DueDate.UTC().Truncate(24 * time.Hour)
	return dueDay.Before(today)
}

// IsDueSoon returns true if the task has a due date within the specified number
// of days (inclusive) and is not overdue or complete.
// Uses day-level comparison (normalizes to midnight UTC).
func (t *Task) IsDueSoon(days int) bool {
	if t.DueDate == nil || t.Status.IsComplete() {
		return false
	}
	today := time.Now().UTC().Truncate(24 * time.Hour)
	dueDay := t.DueDate.UTC().Truncate(24 * time.Hour)
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
// If the task has no due date, returns nil.
// If the task is not recurring, returns nil.
func (t *Task) NextDueDate() *time.Time {
	if t.Recurrence == nil || t.DueDate == nil {
		return nil
	}

	next := calculateNextDate(*t.DueDate, t.Recurrence)
	return &next
}
