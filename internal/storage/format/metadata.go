package format

import (
	"regexp"
	"strings"

	"github.com/sglavoie/tt/internal/models"
)

// pinnedAtFormat uses second-level precision so reordering swaps are never lost
// to minute-level truncation.
const pinnedAtFormat = "2006-01-02 15:04:05"

// metadataRegex matches metadata comments like "<!-- id: uuid, status: todo, priority: 1 -->"
var metadataRegex = regexp.MustCompile(`^<!--\s*(.+?)\s*-->$`)

// MetadataRegex returns the compiled regex for matching metadata comments.
func MetadataRegex() *regexp.Regexp {
	return metadataRegex
}

// LooksLikeMetadata reports whether a comment payload starts with a recognized
// metadata key. This lets the parser distinguish task metadata from arbitrary
// HTML comments that appear at the start of a description.
func LooksLikeMetadata(metaStr string) bool {
	key := extractMetadataKey(metaStr)
	if key == "" {
		return false
	}
	if strings.HasPrefix(key, "score.") {
		return isValidDimensionID(strings.TrimPrefix(key, "score."))
	}
	for _, known := range metadataKeys {
		if key == known {
			return true
		}
	}
	return false
}

// WarningType categorizes the kind of warning for appropriate handling.
type WarningType int

const (
	// WarningValidation indicates a validation issue (e.g., invalid value, out of range).
	// These are informational and shown only in verbose mode.
	WarningValidation WarningType = iota

	// WarningDataCorrection indicates data was automatically corrected (e.g., cleared invalid reference).
	// These are always shown to the user since they represent automatic data changes.
	WarningDataCorrection

	// WarningDeprecation indicates use of deprecated syntax or features.
	// These are shown to encourage migration to new patterns.
	WarningDeprecation
)

// ParseWarning represents a non-fatal issue found during metadata parsing.
type ParseWarning struct {
	Type    WarningType // Category of the warning (determines display behavior)
	TaskID  string      // ID of the task with the issue (may be empty for global warnings)
	Field   string      // Which field had the issue (e.g., "priority", "parent")
	Message string      // Human-readable description of the issue
}

// metadataKeys lists all known metadata keys for smart splitting.
var metadataKeys = []string{
	"id", "status", "priority", "parent", "tags", "created", "updated",
	"completed", "due", "depends", "related", "section", "custom_section", "pinned", "pinned_at", "active", "dimmed", "recur",
	"recur.skipped", "recur.streak", "recur.max_streak", "recur.original_day",
	"attachments", "snoozed", "checklist", "stage", "source", "url",
	"bug.reproducible", "bug.severity", "bug.workaround",
	"idea.category", "idea.effort", "idea.value", "idea.validated",
	"task.effort", "task.worktype",
	"story.points", "story.persona",
	"epic.scope", "epic.milestone", "epic.metrics", "epic.initial_children", "epic.scope_baselined",
	"start", "estimated_days",
}

// MaxDimensionIDLength is the maximum allowed length for a score dimension ID.
const MaxDimensionIDLength = 32

// typeForAttrPrefix returns the expected TaskType for a given attribute prefix.
func typeForAttrPrefix(key string) (models.TaskType, bool) {
	switch {
	case strings.HasPrefix(key, "bug."):
		return models.TaskTypeBug, true
	case strings.HasPrefix(key, "idea."):
		return models.TaskTypeIdea, true
	case strings.HasPrefix(key, "task."):
		return models.TaskTypeTask, true
	case strings.HasPrefix(key, "story."):
		return models.TaskTypeStory, true
	case strings.HasPrefix(key, "epic."):
		return models.TaskTypeEpic, true
	default:
		return "", false
	}
}
