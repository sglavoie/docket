package format

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/util"
)

func addValidationWarning(warnings *[]ParseWarning, field, format string, args ...any) {
	*warnings = append(*warnings, ParseWarning{
		Type:    WarningValidation,
		Field:   field,
		Message: fmt.Sprintf(format, args...),
	})
}

// parseFloatOrWarn parses a float64 value, appending a validation warning on failure.
func parseFloatOrWarn(value, field string, warnings *[]ParseWarning) (float64, bool) {
	v, err := strconv.ParseFloat(value, 64)
	if err != nil {
		addValidationWarning(warnings, field, "invalid %s %q, ignoring: %v", field, value, err)
		return 0, false
	}
	return v, true
}

// splitSemicolonDedup splits a semicolon-delimited string into unique, trimmed,
// non-empty values, preserving insertion order. Used for tags, depends, related.
func splitSemicolonDedup(value string) []string {
	seen := make(map[string]bool)
	var result []string
	for item := range strings.SplitSeq(value, ";") {
		item = strings.TrimSpace(item)
		if item != "" && !seen[item] {
			seen[item] = true
			result = append(result, item)
		}
	}
	return result
}

// parseIntOrWarn parses an integer value, appending a validation warning on failure.
// Returns the parsed value and true on success, or zero and false on failure.
func parseIntOrWarn(value, field string, warnings *[]ParseWarning) (int, bool) {
	v, err := strconv.Atoi(value)
	if err != nil {
		addValidationWarning(warnings, field, "invalid %s %q, ignoring: %v", field, value, err)
		return 0, false
	}
	return v, true
}

func parseTimeOrWarn(value, field string, warnings *[]ParseWarning) (time.Time, bool) {
	if t, ok := ParseTimeField(value); ok {
		return t, true
	}
	addValidationWarning(warnings, field, "invalid %s %q, ignoring", field, value)
	return time.Time{}, false
}

func parseDateOrWarn(value, field string, warnings *[]ParseWarning) (time.Time, bool) {
	if t, err := time.ParseInLocation(util.DateFormat, value, time.Local); err == nil {
		return t, true
	}
	addValidationWarning(warnings, field, "invalid %s %q, ignoring", field, value)
	return time.Time{}, false
}

func parseDueOrWarn(task *models.Task, value string, warnings *[]ParseWarning) {
	if t, err := time.ParseInLocation(util.DateFormat, value, time.Local); err == nil {
		task.DueDate = &t
		return
	}
	if t, err := time.ParseInLocation(util.DateTimeDueFormat, value, time.Local); err == nil {
		task.DueDate = &t
		task.HasTime = true
		return
	}
	addValidationWarning(warnings, "due", "invalid due %q, ignoring", value)
}

// isDecimalDigits reports whether s is non-empty and consists entirely of
// ASCII decimal digits. Used to decide whether a colon-suffix on a code-ref
// path is intended as a line number.
func isDecimalDigits(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}

// looksLikeCodeRefPath reports whether prefix is plausibly the file-path
// portion of a code reference. The empty string is allowed because the writer
// emits "||:N" (sentinel empty path) when LineNumber is non-zero but no
// OriginalPath was provided. Otherwise we require a "/" (any kind of path
// separator) or "." (extension or hidden segment) to distinguish a real path
// from a bare hostname-like token (e.g., "localhost:8080").
func looksLikeCodeRefPath(prefix string) bool {
	if prefix == "" {
		return true
	}
	return strings.ContainsAny(prefix, "/.")
}

// parseAttachments parses a semicolon-delimited attachment string into Attachment structs.
// Uses splitAttachmentEntries so unescaped `;` separates entries while `\;` is
// treated as a literal semicolon inside a single filename or path.
func parseAttachments(value string, warnings *[]ParseWarning) []models.Attachment {
	var result []models.Attachment
	for _, att := range splitAttachmentEntries(value) {
		att = strings.TrimSpace(att)
		if att == "" {
			continue
		}
		var addedAt time.Time
		var originalPath string
		var lineNumber int

		if idx := strings.LastIndex(att, "@"); idx > 0 {
			if epoch, err := strconv.ParseInt(att[idx+1:], 10, 64); err == nil {
				if epoch < 0 {
					*warnings = append(*warnings, ParseWarning{
						Type:    WarningValidation,
						Field:   "attachments",
						Message: fmt.Sprintf("attachment %q has negative timestamp %d, clamping to 0", att[:idx], epoch),
					})
					epoch = 0
				}
				addedAt = time.Unix(epoch, 0)
				att = att[:idx]
			}
		}
		if addedAt.IsZero() {
			addedAt = time.Now()
			*warnings = append(*warnings, ParseWarning{
				Type:    WarningValidation,
				Field:   "attachments",
				Message: fmt.Sprintf("attachment %q has no valid timestamp, defaulting to now", att),
			})
		}

		if origIdx := strings.Index(att, "||"); origIdx > 0 {
			originalPath = att[origIdx+2:]
			att = att[:origIdx]
			if strings.Contains(originalPath, "||") {
				*warnings = append(*warnings, ParseWarning{
					Type:    WarningValidation,
					Field:   "attachments",
					Message: fmt.Sprintf("attachment %q has malformed original path containing '||', data may be corrupt", att),
				})
			}
		}

		if after, ok := strings.CutPrefix(att, "url:"); ok {
			filename := unescapeAttachmentField(after)
			originalPath = unescapeAttachmentField(originalPath)
			if originalPath == "" {
				originalPath = filename
			}
			result = append(result, models.Attachment{
				Filename:     filename,
				IsURL:        true,
				OriginalPath: originalPath,
				AddedAt:      addedAt,
			})
		} else if after, ok := strings.CutPrefix(att, "code:"); ok {
			filename := unescapeAttachmentField(after)
			// Extract the trailing ":N" line-number suffix from the still-escaped
			// originalPath BEFORE unescaping, because the writer appends ":%d"
			// after the escaped path (so any `\;` in the path cannot collide
			// with the line-number colon, but the LastIndex must operate on the
			// pre-unescape string so we don't accidentally see a colon revealed
			// by unescaping `\:` — though our scheme only escapes `\` and `;`).
			//
			// Accept colonIdx >= 0 so a sentinel ":N" (emitted when the
			// writer had no OriginalPath but a non-zero LineNumber) still
			// recovers the line number.
			//
			// Only extract a trailing ":N" suffix as a line number when (a)
			// the suffix is purely decimal digits and (b) the part before the
			// colon looks like a file path (empty for the sentinel, or
			// containing "/" or "."). This prevents corrupting paths whose
			// last segment happens to look like a port (e.g., "localhost:8080")
			// and avoids spurious warnings for Windows-style paths whose only
			// colon belongs to a drive letter (e.g., "C:/Users/foo.go").
			if colonIdx := strings.LastIndex(originalPath, ":"); colonIdx >= 0 {
				suffix := originalPath[colonIdx+1:]
				prefix := originalPath[:colonIdx]
				suffixIsLineNumber := isDecimalDigits(suffix) && looksLikeCodeRefPath(prefix)
				if suffixIsLineNumber {
					ln, _ := strconv.Atoi(suffix)
					if ln <= 0 {
						*warnings = append(*warnings, ParseWarning{
							Type:    WarningValidation,
							Field:   "attachments",
							Message: fmt.Sprintf("code attachment %q has non-positive line number %d, ignoring", filename, ln),
						})
					} else {
						lineNumber = ln
						originalPath = prefix
					}
				}
			}
			originalPath = unescapeAttachmentField(originalPath)
			result = append(result, models.Attachment{
				Filename:     filename,
				IsURL:        false,
				IsCodeRef:    true,
				LineNumber:   lineNumber,
				OriginalPath: originalPath,
				AddedAt:      addedAt,
			})
		} else {
			filename := att
			if after, ok := strings.CutPrefix(att, "file:"); ok {
				// Disambiguation prefix emitted by the writer for plain
				// attachments whose original filename starts with url:/code:/file:.
				filename = after
			}
			filename = unescapeAttachmentField(filename)
			originalPath = unescapeAttachmentField(originalPath)
			result = append(result, models.Attachment{
				Filename:     filename,
				IsURL:        false,
				OriginalPath: originalPath,
				AddedAt:      addedAt,
			})
		}
	}
	return result
}

func finalizeParsedMetadata(task *models.Task, state *metadataParseState, warnings *[]ParseWarning) {
	if state.recurSubFields.hasAny && task.Recurrence != nil {
		task.Recurrence.Skipped = state.recurSubFields.skipped
		task.Recurrence.Streak = state.recurSubFields.streak
		task.Recurrence.MaxStreak = state.recurSubFields.maxStreak
		task.Recurrence.OriginalDay = state.recurSubFields.originalDay
	}

	if task.Recurrence != nil {
		if err := task.Recurrence.Validate(); err != nil {
			*warnings = append(*warnings, ParseWarning{
				Type:    WarningValidation,
				Field:   "recur",
				Message: fmt.Sprintf("invalid recurrence pattern: %v", err),
			})
		}
	}

	if state.recurSubFields.hasAny && task.Recurrence == nil {
		*warnings = append(*warnings, ParseWarning{
			Type:    WarningValidation,
			Field:   "recur",
			Message: "recurrence sub-fields (recur.skipped/streak/max_streak) found without a recur field; ignoring",
		})
	}

	// Derive the canonical stored WorkflowState from the accumulated schema-v1
	// lifecycle values when the document did not carry a schema-v2 `state:` field
	// (the state codec already stored it, and set stateSeen, in that case). This
	// is the loader upgrade that lets a schema-v1 task load into the canonical
	// state model even though the writer no longer emits the legacy lifecycle
	// keys. A non-GTD section is normalized to the default active bucket (empty),
	// mirroring the retired rule that custom grouping now lives in tags.
	if !state.stateSeen {
		section := state.legacy.section
		if section != "" && !isLegacyGTDSection(section) {
			*warnings = append(*warnings, ParseWarning{
				Type:  WarningDataCorrection,
				Field: "section",
				Message: fmt.Sprintf(
					"task #%d had non-GTD section %q; ignoring it because custom grouping now uses tags",
					task.ShortID,
					section,
				),
			})
			section = ""
		}
		task.WorkflowState = deriveLegacyWorkflowState(state.legacy.status, section, state.legacy.stage)
	}
}

// LegacyLifecycle holds the retired schema-v1 lifecycle values a task's metadata
// carried, for the loader-level migration audit. All fields are zero and Present
// is false for a schema-v2 (`state:`) document, which carries none of these keys.
type LegacyLifecycle struct {
	Status  string
	Section string
	Stage   string
	Present bool // any retained lifecycle key (status/section/stage) was seen
}

func legacyLifecycleFrom(state *metadataParseState) LegacyLifecycle {
	return LegacyLifecycle{
		Status:  string(state.legacy.status),
		Section: state.legacy.section,
		Stage:   state.legacy.stage,
		Present: state.legacy.statusSeen || state.legacy.sectionSeen ||
			state.legacy.stageSeen,
	}
}

// ParseMetadata parses the metadata string and sets task fields.
// Returns any warnings encountered during parsing.
func ParseMetadata(task *models.Task, metaStr string) []ParseWarning {
	warnings, _ := parseMetadataInternal(task, metaStr)
	return warnings
}

// ParseMetadataCapturingLegacy is ParseMetadata plus the retired schema-v1
// lifecycle values the metadata carried, so the loader can build the migration
// audit. The canonical stored WorkflowState is set on the task exactly as
// ParseMetadata does.
func ParseMetadataCapturingLegacy(task *models.Task, metaStr string) ([]ParseWarning, LegacyLifecycle) {
	return parseMetadataInternal(task, metaStr)
}

func parseMetadataInternal(task *models.Task, metaStr string) ([]ParseWarning, LegacyLifecycle) {
	var warnings []ParseWarning
	state := metadataParseState{}

	// Parse key-value pairs, handling values that may contain commas (like recur: 1w:mon,wed,fri)
	// Format: "key1: value1, key2: value2, ..."
	// Strategy: find known keys and extract their values until the next known key
	pairs := splitMetadataPairs(metaStr)
	for _, pair := range pairs {
		parts := strings.SplitN(strings.TrimSpace(pair), ":", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		metadataFieldRegistry.parse(task, key, value, &state, &warnings)
	}

	finalizeParsedMetadata(task, &state, &warnings)
	return warnings, legacyLifecycleFrom(&state)
}
