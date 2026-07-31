package models

import (
	"fmt"
	"slices"
	"strings"
)

// TaskMarker is a lightweight visual task kind derived from one reserved tag.
// It is intentionally not a persisted task field: the canonical representation
// remains the raw tag itself.
type TaskMarker string

const (
	TaskMarkerEpic  TaskMarker = "epic"
	TaskMarkerStory TaskMarker = "story"
	TaskMarkerBug   TaskMarker = "bug"
	TaskMarkerIdea  TaskMarker = "idea"
)

var taskMarkers = []TaskMarker{
	TaskMarkerEpic,
	TaskMarkerStory,
	TaskMarkerBug,
	TaskMarkerIdea,
}

// AllTaskMarkers returns the canonical marker vocabulary in display order.
func AllTaskMarkers() []TaskMarker {
	return slices.Clone(taskMarkers)
}

// ParseTaskMarker parses a marker name case-insensitively.
func ParseTaskMarker(value string) (TaskMarker, bool) {
	marker := TaskMarker(strings.ToLower(strings.TrimSpace(value)))
	return marker, slices.Contains(taskMarkers, marker)
}

// ParseTaskMarkerTag recognizes canonical raw marker tags and the four exact
// namespaced spellings emitted by earlier schema-5 builds.
func ParseTaskMarkerTag(tag string) (TaskMarker, bool) {
	normalized := strings.ToLower(strings.TrimSpace(strings.TrimPrefix(tag, "#")))
	if marker, ok := ParseTaskMarker(normalized); ok {
		return marker, true
	}
	legacy, ok := strings.CutPrefix(normalized, "type:")
	if !ok {
		return "", false
	}
	return ParseTaskMarker(legacy)
}

// NormalizeTaskTag applies ordinary tag normalization plus compatibility
// canonicalization for legacy semantic task-marker tags.
func NormalizeTaskTag(raw string) (string, error) {
	tag := NormalizeTag(raw)
	if err := ValidateTag(tag); err != nil {
		return "", err
	}
	if marker, ok := ParseTaskMarkerTag(tag); ok {
		return marker.String(), nil
	}
	return tag, nil
}

// NormalizeTaskTags validates, canonicalizes, and de-duplicates a complete tag
// set. Distinct task markers are mutually exclusive.
func NormalizeTaskTags(tags []string) ([]string, error) {
	normalized := make([]string, 0, len(tags))
	seen := make(map[string]bool, len(tags))
	var selected TaskMarker
	for _, raw := range tags {
		tag, err := NormalizeTaskTag(raw)
		if err != nil {
			return nil, err
		}
		if marker, ok := ParseTaskMarker(tag); ok {
			if selected != "" && selected != marker {
				return nil, fmt.Errorf("task can have only one type marker (%s and %s conflict)", selected, marker)
			}
			selected = marker
		}
		if seen[tag] {
			continue
		}
		seen[tag] = true
		normalized = append(normalized, tag)
	}
	return normalized, nil
}

// TaskMarkerFromTags returns the task's semantic marker, if present. Valid
// persisted tasks have at most one; the first match keeps rendering defensive
// for caller-constructed tasks that have not passed validation.
func TaskMarkerFromTags(tags []string) (TaskMarker, bool) {
	for _, tag := range tags {
		if marker, ok := ParseTaskMarkerTag(tag); ok {
			return marker, true
		}
	}
	return "", false
}

// Marker returns the semantic marker derived from the task's tags.
func (t *Task) Marker() (TaskMarker, bool) {
	if t == nil {
		return "", false
	}
	return TaskMarkerFromTags(t.Tags)
}

// Icon returns the compact row/preview prefix for a task marker.
func (m TaskMarker) Icon() string {
	switch m {
	case TaskMarkerEpic:
		return "E"
	case TaskMarkerStory:
		return "S"
	case TaskMarkerBug:
		return "B"
	case TaskMarkerIdea:
		return "I"
	default:
		return ""
	}
}

func (m TaskMarker) String() string { return string(m) }
