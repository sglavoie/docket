package models

import (
	"time"
)

// ActivityEntry records a single field change in a task's activity log.
type ActivityEntry struct {
	Timestamp time.Time `json:"timestamp"`
	Field     string    `json:"field"`     // e.g., "status", "priority", "tags", "created"
	OldValue  string    `json:"old_value"` // empty for "created" entries
	NewValue  string    `json:"new_value"` // empty for "created" entries
}

// RecordActivity appends an activity entry to the task's log.
func (t *Task) RecordActivity(field, oldValue, newValue string) {
	t.Activity = append(t.Activity, &ActivityEntry{
		Timestamp: time.Now(),
		Field:     field,
		OldValue:  oldValue,
		NewValue:  newValue,
	})
}

// RecordCreated appends a "created" activity entry.
func (t *Task) RecordCreated() {
	t.Activity = append(t.Activity, &ActivityEntry{
		Timestamp: t.CreatedAt,
		Field:     "created",
	})
}

// PruneActivity keeps only the newest maxEntries entries.
func (t *Task) PruneActivity(maxEntries int) {
	if maxEntries <= 0 || len(t.Activity) <= maxEntries {
		return
	}
	t.Activity = t.Activity[len(t.Activity)-maxEntries:]
}
