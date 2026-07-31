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

// NormalizeTag trims leading/trailing whitespace and strips a single leading
// '#' from a tag. Tag rendering always prepends '#', so an input like
// "#backend" must collapse to "backend" to avoid "##backend" on screen and to
// stay consistent with the TUI tag adder (commit 086f1321) and quick-capture
// inline `#tag` syntax. All tag operations should normalize before storing or
// comparing.
func NormalizeTag(tag string) string {
	tag = strings.TrimSpace(tag)
	tag = strings.TrimPrefix(tag, "#")
	return strings.TrimSpace(tag)
}

// ValidateTag checks that a normalized tag is non-empty, not too long, and
// contains no invalid characters.
func ValidateTag(tag string) error {
	if strings.TrimSpace(tag) == "" {
		return fmt.Errorf("tag cannot be empty")
	}
	if tag != NormalizeTag(tag) {
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
	tag, err := NormalizeTaskTag(tag)
	if err != nil {
		return
	}
	if marker, semantic := ParseTaskMarker(tag); semantic {
		if current, exists := t.Marker(); exists && current != marker {
			return
		}
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
	if normalized, err := NormalizeTaskTag(tag); err == nil {
		tag = normalized
	} else {
		tag = NormalizeTag(tag)
	}
	var removed bool
	for _, existing := range slices.Clone(t.Tags) {
		normalized, err := NormalizeTaskTag(existing)
		if err == nil && normalized == tag {
			t.Tags, removed = util.SliceRemove(t.Tags, existing)
			break
		}
	}
	if removed {
		t.UpdatedAt = time.Now()
	}
}

// HasTag returns true if the task has the given tag. The tag is normalized
// (trimmed) before comparison. An empty/whitespace-only tag always returns
// false so a missing or unset filter argument cannot accidentally match a
// malformed empty tag that bypassed validation.
func (t *Task) HasTag(tag string) bool {
	normalized, err := NormalizeTaskTag(tag)
	if err != nil || normalized == "" {
		return false
	}
	for _, existing := range t.Tags {
		candidate, candidateErr := NormalizeTaskTag(existing)
		if candidateErr == nil && candidate == normalized {
			return true
		}
	}
	return false
}
