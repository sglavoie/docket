package models

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/sglavoie/tt/internal/util"
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
	var added bool
	t.Tags, added = util.SliceAdd(t.Tags, tag)
	if added {
		t.UpdatedAt = time.Now()
	}
}

// RemoveTag removes a tag if present.
func (t *Task) RemoveTag(tag string) {
	var removed bool
	t.Tags, removed = util.SliceRemove(t.Tags, tag)
	if removed {
		t.UpdatedAt = time.Now()
	}
}

// HasTag returns true if the task has the given tag.
func (t *Task) HasTag(tag string) bool {
	return slices.Contains(t.Tags, tag)
}
