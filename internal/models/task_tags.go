package models

import (
	"slices"
	"time"
)

// AddTag adds a tag if not already present.
func (t *Task) AddTag(tag string) {
	if slices.Contains(t.Tags, tag) {
		return
	}
	t.Tags = append(t.Tags, tag)
	t.UpdatedAt = time.Now()
}

// RemoveTag removes a tag if present.
func (t *Task) RemoveTag(tag string) {
	if i := slices.Index(t.Tags, tag); i >= 0 {
		t.Tags = slices.Delete(t.Tags, i, i+1)
		t.UpdatedAt = time.Now()
	}
}

// HasTag returns true if the task has the given tag.
func (t *Task) HasTag(tag string) bool {
	return slices.Contains(t.Tags, tag)
}
