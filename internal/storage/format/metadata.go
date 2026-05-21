package format

import (
	"regexp"
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
	return metadataFieldRegistry.recognizesKey(extractMetadataKey(metaStr))
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

// MaxDimensionIDLength is the maximum allowed length for a score dimension ID.
const MaxDimensionIDLength = 32
