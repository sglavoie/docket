package models

import "time"

// SetSection assigns a GTD section to the task.
// Built-in GTD sections (Inbox, Next, Waiting, Someday, Review, Standby,
// Completed) and the empty string (Active view) are stored in Section.
// Non-GTD names are ignored; custom grouping now lives in tags.
func (t *Task) SetSection(name string) {
	oldSection := t.Section
	if IsGTDSection(name) || name == "" {
		t.Section = name
		if t.Section != oldSection {
			t.UpdatedAt = time.Now()
		}
	}
}

// InSection reports whether the task belongs to the given section.
func (t *Task) InSection(name string) bool {
	return t.Section == name
}
