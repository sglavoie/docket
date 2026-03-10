package models

import (
	"fmt"
	"slices"
	"strings"
	"time"
)

// MaxTagLength is the maximum allowed length for a tag.
const MaxTagLength = 100

// ValidateTag checks that a tag is non-empty, not too long, and contains no invalid characters.
func ValidateTag(tag string) error {
	if strings.TrimSpace(tag) == "" {
		return fmt.Errorf("tag cannot be empty")
	}
	if len(tag) > MaxTagLength {
		return fmt.Errorf("tag %q exceeds maximum length of %d characters", tag, MaxTagLength)
	}
	if strings.ContainsAny(tag, ",;\n\r") {
		return fmt.Errorf("tag %q contains invalid characters (cannot contain commas, semicolons, or newlines)", tag)
	}
	return nil
}

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
