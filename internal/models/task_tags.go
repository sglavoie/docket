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

// normalizeTag trims leading/trailing whitespace from a tag.
// All tag operations should normalize before storing or comparing.
func normalizeTag(tag string) string {
	return strings.TrimSpace(tag)
}

// validateTag checks that a tag is non-empty, not too long, and contains no invalid characters.
func validateTag(tag string) error {
	if strings.TrimSpace(tag) == "" {
		return fmt.Errorf("tag cannot be empty")
	}
	if tag != normalizeTag(tag) {
		return fmt.Errorf("tag %q has leading or trailing whitespace", tag)
	}
	if len(tag) > MaxTagLength {
		return fmt.Errorf("tag %q exceeds maximum length of %d characters", tag, MaxTagLength)
	}
	if strings.ContainsAny(tag, ",;\n\r") {
		return fmt.Errorf("tag %q contains invalid characters (cannot contain commas, semicolons, or newlines)", tag)
	}
	return nil
}

// AddTag adds a tag if not already present. The tag is normalized (trimmed)
// before storing. Invalid tags are silently ignored.
func (t *Task) AddTag(tag string) {
	tag = normalizeTag(tag)
	if validateTag(tag) != nil {
		return
	}
	var added bool
	t.Tags, added = util.SliceAdd(t.Tags, tag)
	if added {
		t.UpdatedAt = time.Now()
	}
}

// RemoveTag removes a tag if present. The tag is normalized (trimmed)
// before comparison.
func (t *Task) RemoveTag(tag string) {
	tag = normalizeTag(tag)
	var removed bool
	t.Tags, removed = util.SliceRemove(t.Tags, tag)
	if removed {
		t.UpdatedAt = time.Now()
	}
}

// HasTag returns true if the task has the given tag. The tag is normalized
// (trimmed) before comparison. An empty/whitespace-only tag always returns
// false so a missing or unset filter argument cannot accidentally match a
// malformed empty tag that bypassed validation.
func (t *Task) HasTag(tag string) bool {
	normalized := normalizeTag(tag)
	if normalized == "" {
		return false
	}
	return slices.Contains(t.Tags, normalized)
}
