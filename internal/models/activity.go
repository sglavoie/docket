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

// PruneActivity keeps only the newest maxEntries entries. The "created" entry
// (if present) is always preserved as the first entry.
func (t *Task) PruneActivity(maxEntries int) {
	if maxEntries <= 0 || len(t.Activity) <= maxEntries {
		return
	}

	var createdEntry *ActivityEntry
	for _, entry := range t.Activity {
		if entry != nil && entry.Field == "created" {
			createdEntry = entry
			break
		}
	}

	if createdEntry == nil {
		t.Activity = t.Activity[len(t.Activity)-maxEntries:]
		return
	}

	keep := maxEntries - 1
	if keep <= 0 {
		t.Activity = []*ActivityEntry{createdEntry}
		return
	}

	tail := make([]*ActivityEntry, 0, keep)
	for i := len(t.Activity) - 1; i >= 0 && len(tail) < keep; i-- {
		if t.Activity[i] == createdEntry {
			continue
		}
		tail = append(tail, t.Activity[i])
	}

	pruned := make([]*ActivityEntry, 0, len(tail)+1)
	pruned = append(pruned, createdEntry)
	for i := len(tail) - 1; i >= 0; i-- {
		pruned = append(pruned, tail[i])
	}
	t.Activity = pruned
}
