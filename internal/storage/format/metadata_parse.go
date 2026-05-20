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

// ParseMetadata parses the metadata string and sets task fields.
// Returns any warnings encountered during parsing.
func ParseMetadata(task *models.Task, metaStr string) []ParseWarning { //nolint:gocyclo
	var warnings []ParseWarning

	// Buffer recurrence sub-fields so they work regardless of key order.
	// If "recur" appears before its sub-fields, they're applied immediately.
	// If sub-fields appear first, they're buffered and applied after all keys are parsed.
	var recurSubFields struct {
		skipped     int
		streak      int
		maxStreak   int
		originalDay int
		hasAny      bool
	}

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

		// Gate type-specific attributes on task type
		if expectedType, ok := typeForAttrPrefix(key); ok && task.Type != expectedType {
			warnings = append(warnings, ParseWarning{
				Type:    WarningValidation,
				Field:   key,
				Message: fmt.Sprintf("%s attribute on %s task, skipping", key, task.Type),
			})
			continue
		}

		switch key {
		case "id":
			task.ID = value
		case "status":
			if status, ok := models.ParseTaskStatus(value); ok {
				task.Status = status
			} else {
				addValidationWarning(&warnings, "status", "invalid status %q, using default", value)
			}
		case "priority":
			p, err := strconv.Atoi(value)
			if err != nil {
				addValidationWarning(&warnings, "priority", "invalid priority %q, using default", value)
			} else if err := models.ValidatePriority(p, false); err != nil {
				addValidationWarning(&warnings, "priority", "priority %d out of range (%d-%d), using default", p, models.PriorityMin, models.PriorityMax)
			} else {
				task.Priority = p
			}
		case "parent":
			if value != "" {
				task.ParentID = &value
			}
		case "tags":
			task.Tags = append(task.Tags, splitSemicolonDedup(value)...)
		case "created":
			if t, ok := parseTimeOrWarn(value, "created", &warnings); ok {
				task.CreatedAt = t
			}
		case "updated":
			if t, ok := parseTimeOrWarn(value, "updated", &warnings); ok {
				task.UpdatedAt = t
			}
		case "completed":
			if t, ok := parseTimeOrWarn(value, "completed", &warnings); ok {
				task.CompletedAt = &t
			}
		case "due":
			parseDueOrWarn(task, value, &warnings)
		case "start":
			if t, ok := parseDateOrWarn(value, "start", &warnings); ok {
				task.StartDate = &t
			}
		case "estimated_days":
			if v, ok := parseFloatOrWarn(value, "estimated_days", &warnings); ok && v > 0 {
				task.EstimatedDays = v
			}
		case "depends":
			task.DependsOn = append(task.DependsOn, splitSemicolonDedup(value)...)
		case "related":
			task.RelatedTo = append(task.RelatedTo, splitSemicolonDedup(value)...)
		case "section":
			task.Section = value
		case "custom_section":
			task.CustomSection = value
		case "pinned":
			task.Pinned = value == "true"
		case "pinned_at":
			if t, err := time.ParseInLocation(pinnedAtFormat, value, time.Local); err == nil {
				task.PinnedAt = &t
			} else if t, ok := ParseTimeField(value); ok {
				task.PinnedAt = &t
			}
		case "active":
			task.Active = value == "true"
		case "dimmed":
			task.Dimmed = value == "true"
		case "recur":
			if pattern := models.ParseRecurrence(value); pattern != nil {
				task.Recurrence = pattern
			}
		// Buffer recurrence sub-fields for order-independent parsing.
		// Applied to task.Recurrence after all keys are processed.
		case "recur.skipped":
			if v, ok := parseIntOrWarn(value, key, &warnings); ok {
				recurSubFields.skipped = v
				recurSubFields.hasAny = true
			}
		case "recur.streak":
			if v, ok := parseIntOrWarn(value, key, &warnings); ok {
				recurSubFields.streak = v
				recurSubFields.hasAny = true
			}
		case "recur.max_streak":
			if v, ok := parseIntOrWarn(value, key, &warnings); ok {
				recurSubFields.maxStreak = v
				recurSubFields.hasAny = true
			}
		case "recur.original_day":
			if v, ok := parseIntOrWarn(value, key, &warnings); ok {
				recurSubFields.originalDay = v
				recurSubFields.hasAny = true
			}
		case "snoozed":
			if t, ok := parseDateOrWarn(value, "snoozed", &warnings); ok {
				task.SnoozedUntil = &t
			}
		case "attachments":
			task.Attachments = append(task.Attachments, parseAttachments(value, &warnings)...)
		case "checklist":
			items := splitChecklistItems(value)
			for _, item := range items {
				item = strings.TrimSpace(item)
				if item == "" {
					continue
				}
				done := false
				text := item
				if strings.HasSuffix(item, "[x]") {
					done = true
					text = strings.TrimSuffix(item, "[x]")
				} else if strings.HasSuffix(item, "[ ]") {
					done = false
					text = strings.TrimSuffix(item, "[ ]")
				}
				text = unescapeChecklistText(strings.TrimSpace(text))
				if text != "" {
					task.Checklist = append(task.Checklist, models.ChecklistItem{Text: text, Done: done})
				}
			}
		case "stage":
			task.Stage = value
		case "source":
			task.Source = value
		case "url":
			task.URL = value
		case "bug.reproducible":
			reproducible := models.BugReproducible(value)
			if !reproducible.IsValid() {
				addValidationWarning(&warnings, "bug.reproducible", "invalid bug.reproducible %q, ignoring", value)
				break
			}
			task.EnsureBugAttrs().Reproducible = reproducible
		case "bug.severity":
			severity := models.BugSeverity(value)
			if !severity.IsValid() {
				addValidationWarning(&warnings, "bug.severity", "invalid bug.severity %q, ignoring", value)
				break
			}
			task.EnsureBugAttrs().Severity = severity
		case "bug.workaround":
			task.EnsureBugAttrs().Workaround = unescapeNewlines(value)
		case "idea.category":
			category := models.IdeaCategory(value)
			if !category.IsValid() {
				addValidationWarning(&warnings, "idea.category", "invalid idea.category %q, ignoring", value)
				break
			}
			task.EnsureIdeaAttrs().Category = category
		case "idea.effort":
			effort := models.EffortEstimate(value)
			if !effort.IsValid() {
				addValidationWarning(&warnings, "idea.effort", "invalid idea.effort %q, ignoring", value)
				break
			}
			task.EnsureIdeaAttrs().Effort = effort
		case "idea.value":
			ideaValue := models.IdeaValue(value)
			if !ideaValue.IsValid() {
				addValidationWarning(&warnings, "idea.value", "invalid idea.value %q, ignoring", value)
				break
			}
			task.EnsureIdeaAttrs().Value = ideaValue
		case "idea.validated":
			task.EnsureIdeaAttrs().Validated = value == "true"
		case "task.effort":
			effort := models.EffortEstimate(value)
			if !effort.IsValid() {
				addValidationWarning(&warnings, "task.effort", "invalid task.effort %q, ignoring", value)
				break
			}
			task.EnsureTaskAttrs().Effort = effort
		case "task.worktype":
			workType := models.WorkType(value)
			if !workType.IsValid() {
				addValidationWarning(&warnings, "task.worktype", "invalid task.worktype %q, ignoring", value)
				break
			}
			task.EnsureTaskAttrs().WorkType = workType
		case "story.points":
			if points, ok := parseIntOrWarn(value, "story.points", &warnings); ok {
				if points < 0 {
					addValidationWarning(&warnings, "story.points", "invalid story.points %d (must be non-negative), ignoring", points)
					break
				}
				task.EnsureStoryAttrs().StoryPoints = points
			}
		case "story.persona":
			task.EnsureStoryAttrs().Persona = unescapeNewlines(value)
		case "epic.scope":
			scope := models.EpicScope(value)
			if !scope.IsValid() {
				addValidationWarning(&warnings, "epic.scope", "invalid epic.scope %q, ignoring", value)
				break
			}
			task.EnsureEpicAttrs().Scope = scope
		case "epic.milestone":
			task.EnsureEpicAttrs().Milestone = value
		case "epic.metrics":
			task.EnsureEpicAttrs().SuccessMetrics = unescapeNewlines(value)
		case "epic.initial_children":
			if v, ok := parseIntOrWarn(value, "epic.initial_children", &warnings); ok && v > 0 {
				task.EnsureEpicAttrs().InitialChildCount = v
			}
		case "epic.scope_baselined":
			if t, ok := parseDateOrWarn(value, "epic.scope_baselined", &warnings); ok {
				task.EnsureEpicAttrs().ScopeBaselinedAt = &t
			}
		default:
			if strings.HasPrefix(key, "score.") {
				dimID := key[6:]
				if !isValidDimensionID(dimID) {
					warnings = append(warnings, ParseWarning{
						Type:    WarningValidation,
						Field:   key,
						Message: fmt.Sprintf("invalid score dimension ID '%s' (must be 1-%d alphanumeric/underscore chars), skipping", dimID, MaxDimensionIDLength),
					})
				} else if v, err := strconv.Atoi(value); err == nil {
					if task.ScoreDimensions == nil {
						task.ScoreDimensions = make(map[string]int)
					}
					task.ScoreDimensions[dimID] = v
				} else {
					warnings = append(warnings, ParseWarning{
						Type:    WarningValidation,
						Field:   key,
						Message: fmt.Sprintf("invalid score value '%s' for dimension '%s', skipping", value, dimID),
					})
				}
			}
		}
	}

	if recurSubFields.hasAny && task.Recurrence != nil {
		task.Recurrence.Skipped = recurSubFields.skipped
		task.Recurrence.Streak = recurSubFields.streak
		task.Recurrence.MaxStreak = recurSubFields.maxStreak
		task.Recurrence.OriginalDay = recurSubFields.originalDay
	}

	if task.Recurrence != nil {
		if err := task.Recurrence.Validate(); err != nil {
			warnings = append(warnings, ParseWarning{
				Type:    WarningValidation,
				Field:   "recur",
				Message: fmt.Sprintf("invalid recurrence pattern: %v", err),
			})
		}
	}

	if recurSubFields.hasAny && task.Recurrence == nil {
		warnings = append(warnings, ParseWarning{
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

	return warnings
}
