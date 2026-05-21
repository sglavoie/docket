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

// parseAttachments parses a semicolon-delimited attachment string into Attachment structs.
func parseAttachments(value string, warnings *[]ParseWarning) []models.Attachment {
	var result []models.Attachment
	for att := range strings.SplitSeq(value, ";") {
		att = strings.TrimSpace(att)
		if att == "" {
			continue
		}
		var addedAt time.Time
		var originalPath string
		var lineNumber int

		if idx := strings.LastIndex(att, "@"); idx > 0 {
			if epoch, err := strconv.ParseInt(att[idx+1:], 10, 64); err == nil {
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
			filename := after
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
			filename := after
			if colonIdx := strings.LastIndex(originalPath, ":"); colonIdx > 0 {
				suffix := originalPath[colonIdx+1:]
				if ln, err := strconv.Atoi(suffix); err != nil {
					*warnings = append(*warnings, ParseWarning{
						Type:    WarningValidation,
						Field:   "attachments",
						Message: fmt.Sprintf("code attachment %q has invalid line number %q, ignoring", filename, suffix),
					})
				} else if ln <= 0 {
					*warnings = append(*warnings, ParseWarning{
						Type:    WarningValidation,
						Field:   "attachments",
						Message: fmt.Sprintf("code attachment %q has non-positive line number %d, ignoring", filename, ln),
					})
				} else {
					lineNumber = ln
					originalPath = originalPath[:colonIdx]
				}
			}
			result = append(result, models.Attachment{
				Filename:     filename,
				IsURL:        false,
				IsCodeRef:    true,
				LineNumber:   lineNumber,
				OriginalPath: originalPath,
				AddedAt:      addedAt,
			})
		} else {
			result = append(result, models.Attachment{
				Filename:     att,
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

	if task.Section != "" && !models.IsGTDSection(task.Section) {
		if task.CustomSection == "" {
			// Migrate non-GTD section value to CustomSection.
			task.CustomSection = task.Section
		}
		// Clear the non-GTD value from Section regardless. When CustomSection
		// is already set, it takes precedence; leaving a stale non-GTD value
		// in Section would confuse downstream code that reads Section directly.
		task.Section = ""
	}
}

// ParseMetadata parses the metadata string and sets task fields.
// Returns any warnings encountered during parsing.
func ParseMetadata(task *models.Task, metaStr string) []ParseWarning {
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
	return warnings
}
