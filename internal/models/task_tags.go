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

// NormalizeTag returns the one canonical spelling accepted by both task and
// project tags: lowercase ASCII kebab case. A leading # is presentation syntax,
// and every run outside [a-z0-9] becomes one dash. Tags that contain no ASCII
// letter or digit normalize to the empty string and are rejected by ValidateTag.
func NormalizeTag(tag string) string {
	tag = strings.TrimSpace(tag)
	tag = strings.TrimPrefix(tag, "#")
	tag = strings.ToLower(strings.TrimSpace(tag))
	var normalized strings.Builder
	dashPending := false
	for _, r := range tag {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			if dashPending && normalized.Len() > 0 {
				normalized.WriteByte('-')
			}
			normalized.WriteRune(r)
			dashPending = false
			continue
		}
		// Treat every punctuation, separator, and non-ASCII rune as one kebab
		// boundary.
		dashPending = true
	}
	return normalized.String()
}

// ValidateTag checks that a stored tag is already canonical lowercase ASCII
// kebab case and within the durable length bound.
func ValidateTag(tag string) error {
	if tag == "" {
		return fmt.Errorf("tag cannot be empty")
	}
	if len(tag) > MaxTagLength {
		return fmt.Errorf("tag %q exceeds maximum length of %d characters", tag, MaxTagLength)
	}
	if tag != NormalizeTag(tag) {
		return fmt.Errorf("tag %q is not canonical lowercase kebab case", tag)
	}
	return nil
}

// NormalizeTaskTag applies ordinary tag normalization and validation. Every
// valid tag is independent; semantic presentation tags receive no storage-time
// canonicalization or exclusivity treatment.
func NormalizeTaskTag(raw string) (string, error) {
	tag := NormalizeTag(raw)
	if err := ValidateTag(tag); err != nil {
		return "", err
	}
	return tag, nil
}

// NormalizeTaskTags validates and de-duplicates a complete ordinary tag set.
func NormalizeTaskTags(tags []string) ([]string, error) {
	normalized := make([]string, 0, len(tags))
	seen := make(map[string]bool, len(tags))
	for _, raw := range tags {
		tag, err := NormalizeTaskTag(raw)
		if err != nil {
			return nil, err
		}
		if seen[tag] {
			continue
		}
		seen[tag] = true
		normalized = append(normalized, tag)
	}
	return normalized, nil
}

// ReplaceTag rewrites every occurrence of the canonical tag from as to and
// returns the resulting set plus whether it differs from tags. Both sides are
// compared on their canonical spelling, so a stored variant renames like the
// value a filter would have matched. A rename onto a tag the task already
// carries merges the two: the destination keeps the earliest position of either
// membership and appears exactly once, while every unrelated tag keeps its own
// position and spelling.
//
// It is deliberately a pure list transform rather than a Task method: AddTag and
// RemoveTag stamp UpdatedAt, and a vocabulary rename is not task activity.
func ReplaceTag(tags []string, from, to string) ([]string, bool) {
	renamed := make([]string, 0, len(tags))
	seen := make(map[string]bool, len(tags))
	for _, existing := range tags {
		key := existing
		if normalized, err := NormalizeTaskTag(existing); err == nil {
			key = normalized
		}
		value := existing
		if key == from {
			key, value = to, to
		}
		if seen[key] {
			continue
		}
		seen[key] = true
		renamed = append(renamed, value)
	}
	if slices.Equal(renamed, tags) {
		return tags, false
	}
	return renamed, true
}

// AddTag adds a tag if not already present. The tag is normalized (trimmed)
// before storing. Invalid tags are silently ignored.
func (t *Task) AddTag(tag string) {
	t.AddTagAt(tag, time.Now())
}

// AddTagAt is AddTag touching the task at now when the tag is new.
func (t *Task) AddTagAt(tag string, now time.Time) {
	tag, err := NormalizeTaskTag(tag)
	if err != nil {
		return
	}
	var added bool
	t.Tags, added = util.SliceAdd(t.Tags, tag)
	if added {
		t.UpdatedAt = now
	}
}

// RemoveTag removes a tag if present. The tag is normalized (trimmed)
// before comparison.
func (t *Task) RemoveTag(tag string) {
	t.RemoveTagAt(tag, time.Now())
}

// RemoveTagAt is RemoveTag touching the task at now when a tag was removed.
func (t *Task) RemoveTagAt(tag string, now time.Time) {
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
		t.UpdatedAt = now
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
