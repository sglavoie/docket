package models

import (
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

// NormalizeTaskTag applies ordinary tag normalization and validation. Marker
// spellings are not canonicalized in the live model: every tag is independent.
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

// TaskMarkerFromTags returns the first recognized legacy visual marker. Tags
// are independent, so callers that still render this compatibility affordance
// must not infer exclusivity from the result.
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
