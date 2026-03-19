package format

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/util"
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
	"completed", "due", "depends", "related", "section", "custom_section", "pinned", "pinned_at", "active", "recur",
	"recur.skipped", "recur.streak", "recur.max_streak",
	"attachments", "snoozed", "checklist", "stage", "source", "url",
	// Type-specific attribute keys
	"bug.reproducible", "bug.severity", "bug.workaround",
	"idea.category", "idea.effort", "idea.value", "idea.validated",
	"task.effort", "task.worktype",
	"story.points", "story.persona",
	"epic.scope", "epic.milestone", "epic.metrics",
}

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

// ParseMetadata parses the metadata string and sets task fields.
// Returns any warnings encountered during parsing.
func ParseMetadata(task *models.Task, metaStr string) []ParseWarning {
	var warnings []ParseWarning

	// Buffer recurrence sub-fields so they work regardless of key order.
	// If "recur" appears before its sub-fields, they're applied immediately.
	// If sub-fields appear first, they're buffered and applied after all keys are parsed.
	var recurSubFields struct {
		skipped   int
		streak    int
		maxStreak int
		hasAny    bool
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
			}
		case "priority":
			p, err := strconv.Atoi(value)
			if err != nil {
				warnings = append(warnings, ParseWarning{
					Type:    WarningValidation,
					Field:   "priority",
					Message: fmt.Sprintf("invalid priority '%s', using default", value),
				})
			} else if p < 1 || p > 5 {
				warnings = append(warnings, ParseWarning{
					Type:    WarningValidation,
					Field:   "priority",
					Message: fmt.Sprintf("priority %d out of range (1-5), using default", p),
				})
			} else {
				task.Priority = p
			}
		case "parent":
			if value != "" {
				task.ParentID = &value
			}
		case "tags":
			tags := strings.Split(value, ";")
			for _, tag := range tags {
				tag = strings.TrimSpace(tag)
				if tag != "" {
					task.Tags = append(task.Tags, tag)
				}
			}
		case "created":
			if t, ok := ParseTimeField(value); ok {
				task.CreatedAt = t
			}
		case "updated":
			if t, ok := ParseTimeField(value); ok {
				task.UpdatedAt = t
			}
		case "completed":
			if t, ok := ParseTimeField(value); ok {
				task.CompletedAt = &t
			}
		case "due":
			if t, err := time.ParseInLocation(util.DateFormat, value, time.Local); err == nil {
				task.DueDate = &t
			}
		case "depends":
			deps := strings.Split(value, ";")
			for _, dep := range deps {
				dep = strings.TrimSpace(dep)
				if dep != "" {
					task.DependsOn = append(task.DependsOn, dep)
				}
			}
		case "related":
			related := strings.Split(value, ";")
			for _, rel := range related {
				rel = strings.TrimSpace(rel)
				if rel != "" {
					task.RelatedTo = append(task.RelatedTo, rel)
				}
			}
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
		case "recur":
			if pattern := models.ParseRecurrence(value); pattern != nil {
				task.Recurrence = pattern
			}
		// Buffer recurrence sub-fields for order-independent parsing.
		// Applied to task.Recurrence after all keys are processed.
		case "recur.skipped":
			if v, err := strconv.Atoi(value); err == nil {
				recurSubFields.skipped = v
				recurSubFields.hasAny = true
			}
		case "recur.streak":
			if v, err := strconv.Atoi(value); err == nil {
				recurSubFields.streak = v
				recurSubFields.hasAny = true
			}
		case "recur.max_streak":
			if v, err := strconv.Atoi(value); err == nil {
				recurSubFields.maxStreak = v
				recurSubFields.hasAny = true
			}
		case "snoozed":
			if t, err := time.ParseInLocation(util.DateFormat, value, time.Local); err == nil {
				task.SnoozedUntil = &t
			}
		case "attachments":
			attachments := strings.Split(value, ";")
			for _, att := range attachments {
				att = strings.TrimSpace(att)
				if att == "" {
					continue
				}
				// Parse timestamp from @epoch suffix if present
				var addedAt time.Time
				var originalPath string
				var lineNumber int

				// First, extract timestamp
				if idx := strings.LastIndex(att, "@"); idx > 0 {
					if epoch, err := strconv.ParseInt(att[idx+1:], 10, 64); err == nil {
						addedAt = time.Unix(epoch, 0)
						att = att[:idx]
					}
				}
				if addedAt.IsZero() {
					addedAt = time.Now()
				}

				// Then, extract original path from the timestamp-stripped string.
				// The "||" delimiter separates filename from original path;
				// filenames/paths containing "||" are rejected at write time
				// (see models.ValidateAttachmentPath).
				if origIdx := strings.Index(att, "||"); origIdx > 0 {
					originalPath = att[origIdx+2:]
					att = att[:origIdx]
				}

				if after, ok := strings.CutPrefix(att, "url:"); ok {
					filename := after
					if originalPath == "" {
						originalPath = filename // URLs: original path same as filename if not specified
					}
					task.Attachments = append(task.Attachments, models.Attachment{
						Filename:     filename,
						IsURL:        true,
						OriginalPath: originalPath,
						AddedAt:      addedAt,
					})
				} else if after, ok := strings.CutPrefix(att, "code:"); ok {
					filename := after
					// Extract line number from originalPath (format: path:line)
					if colonIdx := strings.LastIndex(originalPath, ":"); colonIdx > 0 {
						if ln, err := strconv.Atoi(originalPath[colonIdx+1:]); err == nil && ln > 0 {
							lineNumber = ln
							originalPath = originalPath[:colonIdx]
						}
					}
					task.Attachments = append(task.Attachments, models.Attachment{
						Filename:     filename,
						IsURL:        false,
						IsCodeRef:    true,
						LineNumber:   lineNumber,
						OriginalPath: originalPath,
						AddedAt:      addedAt,
					})
				} else {
					task.Attachments = append(task.Attachments, models.Attachment{
						Filename:     att,
						IsURL:        false,
						OriginalPath: originalPath,
						AddedAt:      addedAt,
					})
				}
			}
		case "checklist":
			// Format: "text1[x];text2[ ];text3[x]" where [x] = done, [ ] = not done
			// Semicolons and [x]/[ ] in text are escaped as \; and \[x\]/\[ \]
			items := splitChecklistItems(value)
			for _, item := range items {
				item = strings.TrimSpace(item)
				if item == "" {
					continue
				}
				done := false
				text := item
				// Check for [x] or [ ] suffix
				if strings.HasSuffix(item, "[x]") {
					done = true
					text = strings.TrimSuffix(item, "[x]")
				} else if strings.HasSuffix(item, "[ ]") {
					done = false
					text = strings.TrimSuffix(item, "[ ]")
				}
				text = unescapeChecklistText(strings.TrimSpace(text))
				if text != "" {
					task.Checklist = append(task.Checklist, models.ChecklistItem{
						Text: text,
						Done: done,
					})
				}
			}

		case "stage":
			task.Stage = value
		case "source":
			task.Source = value
		case "url":
			task.URL = value
			// Bug attributes
		case "bug.reproducible":
			task.EnsureBugAttrs().Reproducible = models.BugReproducible(value)
		case "bug.severity":
			task.EnsureBugAttrs().Severity = models.BugSeverity(value)
		case "bug.workaround":
			task.EnsureBugAttrs().Workaround = unescapeNewlines(value)

		// Idea attributes
		case "idea.category":
			task.EnsureIdeaAttrs().Category = models.IdeaCategory(value)
		case "idea.effort":
			task.EnsureIdeaAttrs().Effort = models.EffortEstimate(value)
		case "idea.value":
			task.EnsureIdeaAttrs().Value = models.IdeaValue(value)
		case "idea.validated":
			task.EnsureIdeaAttrs().Validated = value == "true"

		// Task attributes
		case "task.effort":
			task.EnsureTaskAttrs().Effort = models.EffortEstimate(value)
		case "task.worktype":
			task.EnsureTaskAttrs().WorkType = models.WorkType(value)

		// Story attributes
		case "story.points":
			if points, err := strconv.Atoi(value); err == nil {
				task.EnsureStoryAttrs().StoryPoints = points
			}
		case "story.persona":
			task.EnsureStoryAttrs().Persona = unescapeNewlines(value)

		// Epic attributes
		case "epic.scope":
			task.EnsureEpicAttrs().Scope = models.EpicScope(value)
		case "epic.milestone":
			task.EnsureEpicAttrs().Milestone = value
		case "epic.metrics":
			task.EnsureEpicAttrs().SuccessMetrics = unescapeNewlines(value)

		default:
			// Handle score.* dimension keys
			if strings.HasPrefix(key, "score.") {
				dimID := key[6:] // strip "score." prefix
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
	// Apply buffered recurrence sub-fields now that all keys are parsed
	if recurSubFields.hasAny && task.Recurrence != nil {
		task.Recurrence.Skipped = recurSubFields.skipped
		task.Recurrence.Streak = recurSubFields.streak
		task.Recurrence.MaxStreak = recurSubFields.maxStreak
	}

	// Validate recurrence pattern if present
	if task.Recurrence != nil {
		if err := task.Recurrence.Validate(); err != nil {
			warnings = append(warnings, ParseWarning{
				Type:    WarningValidation,
				Field:   "recur",
				Message: fmt.Sprintf("invalid recurrence pattern: %v", err),
			})
		}
	}

	// Warn on orphaned recurrence sub-fields (recur.skipped/streak/max_streak without recur)
	if recurSubFields.hasAny && task.Recurrence == nil {
		warnings = append(warnings, ParseWarning{
			Type:    WarningValidation,
			Field:   "recur",
			Message: "recurrence sub-fields (recur.skipped/streak/max_streak) found without a recur field; ignoring",
		})
	}

	// Migration: move non-GTD section values to CustomSection
	if task.Section != "" && !models.IsGTDSection(task.Section) && task.CustomSection == "" {
		task.CustomSection = task.Section
		task.Section = ""
	}

	return warnings
}

// splitMetadataPairs splits metadata string into key-value pairs, handling values
// that may contain commas (e.g., "recur: 1w:mon,wed,fri").
// It looks for ", key:" patterns where key is a known metadata key or a "score.*" prefix.
// To prevent free-text values (URLs, descriptions) that happen to contain ", key:"
// patterns from being mis-split, it tracks already-seen keys and only splits on
// keys that haven't appeared yet (since each key appears at most once in metadata).
func splitMetadataPairs(metaStr string) []string {
	var pairs []string
	remaining := metaStr
	seenKeys := make(map[string]bool)

	for len(remaining) > 0 {
		// Extract the key from the current pair so we can track seen keys.
		// The current pair starts with "key: value" or is just "key:" with no value.
		currentKey := extractMetadataKey(remaining)
		if currentKey != "" {
			seenKeys[currentKey] = true
		}

		// Find the earliest occurrence of ", knownKey:" in the remaining string,
		// but only for keys we haven't already seen (to avoid mis-splitting on
		// key-like patterns inside free-text values).
		splitIdx := -1
		for _, key := range metadataKeys {
			if seenKeys[key] {
				continue
			}
			pattern := ", " + key + ":"
			idx := strings.Index(remaining, pattern)
			if idx != -1 && (splitIdx == -1 || idx < splitIdx) {
				splitIdx = idx
			}
		}

		// Also check for ", score.<id>:" wildcard prefix pattern
		if idx := findScoreKeyIndex(remaining); idx != -1 && (splitIdx == -1 || idx < splitIdx) {
			// Extract the score key to check if we've seen it
			scoreKey := extractScoreKey(remaining[idx+2:])
			if !seenKeys[scoreKey] {
				splitIdx = idx
			}
		}

		if splitIdx == -1 {
			// No more splits found, the rest is one pair
			pairs = append(pairs, strings.TrimSpace(remaining))
			break
		}

		// Add the pair before the split point
		pairs = append(pairs, strings.TrimSpace(remaining[:splitIdx]))
		// Continue with the rest (skip the ", ")
		remaining = remaining[splitIdx+2:]
	}

	return pairs
}

// extractMetadataKey returns the key portion from a "key: value" string,
// or empty string if no valid key is found.
func extractMetadataKey(s string) string {
	s = strings.TrimSpace(s)
	colonIdx := strings.Index(s, ":")
	if colonIdx <= 0 {
		return ""
	}
	return strings.TrimSpace(s[:colonIdx])
}

// extractScoreKey returns the full "score.<id>" key from a string starting with "score.<id>:".
func extractScoreKey(s string) string {
	if !strings.HasPrefix(s, "score.") {
		return ""
	}
	rest := s[6:]
	colonIdx := strings.Index(rest, ":")
	if colonIdx <= 0 {
		return ""
	}
	return "score." + rest[:colonIdx]
}

// findScoreKeyIndex finds the index of the first ", score.<id>:" pattern in s.
// Returns -1 if not found. The score dimension ID must be non-empty and contain
// only alphanumeric characters and underscores.
func findScoreKeyIndex(s string) int {
	searchFrom := 0
	for {
		idx := strings.Index(s[searchFrom:], ", score.")
		if idx == -1 {
			return -1
		}
		idx += searchFrom
		// Verify this is a valid score key: ", score.<id>:" where id is non-empty
		rest := s[idx+8:] // skip ", score."
		colonIdx := strings.Index(rest, ":")
		if colonIdx > 0 {
			dimID := rest[:colonIdx]
			if isValidDimensionID(dimID) {
				return idx
			}
		}
		searchFrom = idx + 2
		if searchFrom >= len(s) {
			return -1
		}
	}
}

// MaxDimensionIDLength is the maximum allowed length for a score dimension ID.
const MaxDimensionIDLength = 32

// isValidDimensionID checks that a dimension ID contains only [a-zA-Z0-9_]
// and does not exceed MaxDimensionIDLength.
func isValidDimensionID(id string) bool {
	if id == "" || len(id) > MaxDimensionIDLength {
		return false
	}
	for _, c := range id {
		if (c < 'a' || c > 'z') && (c < 'A' || c > 'Z') && (c < '0' || c > '9') && c != '_' {
			return false
		}
	}
	return true
}

// ParseTimeField attempts to parse a time value in datetime or date-only format.
// Returns the parsed time and true if successful, zero time and false otherwise.
func ParseTimeField(value string) (time.Time, bool) {
	if t, err := time.ParseInLocation(util.DateTimeFormat, value, time.Local); err == nil {
		return t, true
	}
	if t, err := time.ParseInLocation(util.DateFormat, value, time.Local); err == nil {
		return t, true
	}
	return time.Time{}, false
}

// escapeNewlines replaces actual newlines with literal \n for storage in metadata.
func escapeNewlines(s string) string {
	return strings.ReplaceAll(s, "\n", `\n`)
}

// unescapeNewlines replaces literal \n with actual newlines when reading from metadata.
func unescapeNewlines(s string) string {
	return strings.ReplaceAll(s, `\n`, "\n")
}

// escapeChecklistText escapes semicolons and done/undone markers in checklist item text
// so they don't interfere with the storage format delimiters.
func escapeChecklistText(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, ";", `\;`)
	s = strings.ReplaceAll(s, "[x]", `\[x\]`)
	s = strings.ReplaceAll(s, "[ ]", `\[ \]`)
	return s
}

// unescapeChecklistText reverses the escaping done by escapeChecklistText.
func unescapeChecklistText(s string) string {
	s = strings.ReplaceAll(s, `\;`, ";")
	s = strings.ReplaceAll(s, `\[x\]`, "[x]")
	s = strings.ReplaceAll(s, `\[ \]`, "[ ]")
	s = strings.ReplaceAll(s, `\\`, `\`)
	return s
}

// splitChecklistItems splits a checklist value string by unescaped semicolons.
func splitChecklistItems(s string) []string {
	var items []string
	var current strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+1 < len(s) {
			// Escaped character — keep both the backslash and the next char
			current.WriteByte(s[i])
			i++
			current.WriteByte(s[i])
		} else if s[i] == ';' {
			items = append(items, current.String())
			current.Reset()
		} else {
			current.WriteByte(s[i])
		}
	}
	items = append(items, current.String())
	return items
}

// sortedKeys returns the keys of an int map in sorted order.
func sortedKeys(m map[string]int) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// WriteMetadata generates the metadata comment for a task.
func WriteMetadata(task *models.Task) string {
	var parts []string

	if task.ID != "" {
		parts = append(parts, fmt.Sprintf("id: %s", task.ID))
	}

	parts = append(parts, fmt.Sprintf("status: %s", task.Status))
	parts = append(parts, fmt.Sprintf("priority: %d", task.Priority))

	if task.ParentID != nil && *task.ParentID != "" {
		parts = append(parts, fmt.Sprintf("parent: %s", *task.ParentID))
	}

	if len(task.Tags) > 0 {
		parts = append(parts, fmt.Sprintf("tags: %s", strings.Join(task.Tags, ";")))
	}

	if task.Section != "" {
		parts = append(parts, fmt.Sprintf("section: %s", task.Section))
	}

	if task.CustomSection != "" {
		parts = append(parts, fmt.Sprintf("custom_section: %s", task.CustomSection))
	}

	if !task.CreatedAt.IsZero() {
		parts = append(parts, fmt.Sprintf("created: %s", task.CreatedAt.Format(util.DateTimeFormat)))
	}

	if !task.UpdatedAt.IsZero() && !task.UpdatedAt.Equal(task.CreatedAt) {
		parts = append(parts, fmt.Sprintf("updated: %s", task.UpdatedAt.Format(util.DateTimeFormat)))
	}

	if task.CompletedAt != nil {
		parts = append(parts, fmt.Sprintf("completed: %s", task.CompletedAt.Format(util.DateTimeFormat)))
	}

	if task.DueDate != nil {
		parts = append(parts, fmt.Sprintf("due: %s", task.DueDate.Format(util.DateFormat)))
	}

	if len(task.DependsOn) > 0 {
		parts = append(parts, fmt.Sprintf("depends: %s", strings.Join(task.DependsOn, ";")))
	}

	if len(task.RelatedTo) > 0 {
		parts = append(parts, fmt.Sprintf("related: %s", strings.Join(task.RelatedTo, ";")))
	}

	if task.Pinned {
		parts = append(parts, "pinned: true")
	}

	if task.PinnedAt != nil {
		parts = append(parts, fmt.Sprintf("pinned_at: %s", task.PinnedAt.Format(pinnedAtFormat)))
	}

	if task.Active {
		parts = append(parts, "active: true")
	}

	if task.SnoozedUntil != nil {
		parts = append(parts, fmt.Sprintf("snoozed: %s", task.SnoozedUntil.Format(util.DateFormat)))
	}

	if task.Recurrence != nil {
		parts = append(parts, fmt.Sprintf("recur: %s", FormatRecurrence(task.Recurrence)))
		if task.Recurrence.Skipped > 0 {
			parts = append(parts, fmt.Sprintf("recur.skipped: %d", task.Recurrence.Skipped))
		}
		if task.Recurrence.Streak > 0 {
			parts = append(parts, fmt.Sprintf("recur.streak: %d", task.Recurrence.Streak))
		}
		if task.Recurrence.MaxStreak > 0 {
			parts = append(parts, fmt.Sprintf("recur.max_streak: %d", task.Recurrence.MaxStreak))
		}
	}

	if len(task.Attachments) > 0 {
		var attStrings []string
		for _, att := range task.Attachments {
			var attStr string
			if att.IsURL {
				attStr = "url:" + att.Filename
			} else if att.IsCodeRef {
				attStr = "code:" + att.Filename
			} else {
				attStr = att.Filename
			}
			// Append original path (for code refs, include line number)
			if att.OriginalPath != "" {
				if att.IsCodeRef && att.LineNumber > 0 {
					attStr += fmt.Sprintf("||%s:%d", att.OriginalPath, att.LineNumber)
				} else {
					attStr += "||" + att.OriginalPath
				}
			}
			timestamp := fmt.Sprintf("@%d", att.AddedAt.Unix())
			attStrings = append(attStrings, attStr+timestamp)
		}
		parts = append(parts, fmt.Sprintf("attachments: %s", strings.Join(attStrings, ";")))
	}

	if len(task.Checklist) > 0 {
		var checklistStrings []string
		for _, item := range task.Checklist {
			marker := "[ ]"
			if item.Done {
				marker = "[x]"
			}
			checklistStrings = append(checklistStrings, escapeChecklistText(item.Text)+marker)
		}
		parts = append(parts, fmt.Sprintf("checklist: %s", strings.Join(checklistStrings, ";")))
	}

	if task.Stage != "" {
		parts = append(parts, fmt.Sprintf("stage: %s", task.Stage))
	}

	if task.Source != "" {
		parts = append(parts, fmt.Sprintf("source: %s", task.Source))
	}

	if task.URL != "" {
		parts = append(parts, fmt.Sprintf("url: %s", task.URL))
	}

	// Type-specific attributes
	if task.BugAttrs != nil && !task.BugAttrs.IsEmpty() {
		if task.BugAttrs.Reproducible != "" {
			parts = append(parts, fmt.Sprintf("bug.reproducible: %s", task.BugAttrs.Reproducible))
		}
		if task.BugAttrs.Severity != "" {
			parts = append(parts, fmt.Sprintf("bug.severity: %s", task.BugAttrs.Severity))
		}
		if task.BugAttrs.Workaround != "" {
			parts = append(parts, fmt.Sprintf("bug.workaround: %s", escapeNewlines(task.BugAttrs.Workaround)))
		}
	}

	if task.IdeaAttrs != nil && !task.IdeaAttrs.IsEmpty() {
		if task.IdeaAttrs.Category != "" {
			parts = append(parts, fmt.Sprintf("idea.category: %s", task.IdeaAttrs.Category))
		}
		if task.IdeaAttrs.Effort != "" {
			parts = append(parts, fmt.Sprintf("idea.effort: %s", task.IdeaAttrs.Effort))
		}
		if task.IdeaAttrs.Value != "" {
			parts = append(parts, fmt.Sprintf("idea.value: %s", task.IdeaAttrs.Value))
		}
		if task.IdeaAttrs.Validated {
			parts = append(parts, "idea.validated: true")
		}
	}

	if task.TaskAttrs != nil && !task.TaskAttrs.IsEmpty() {
		if task.TaskAttrs.Effort != "" {
			parts = append(parts, fmt.Sprintf("task.effort: %s", task.TaskAttrs.Effort))
		}
		if task.TaskAttrs.WorkType != "" {
			parts = append(parts, fmt.Sprintf("task.worktype: %s", task.TaskAttrs.WorkType))
		}
	}

	if task.StoryAttrs != nil && !task.StoryAttrs.IsEmpty() {
		if task.StoryAttrs.StoryPoints > 0 {
			parts = append(parts, fmt.Sprintf("story.points: %d", task.StoryAttrs.StoryPoints))
		}
		if task.StoryAttrs.Persona != "" {
			parts = append(parts, fmt.Sprintf("story.persona: %s", escapeNewlines(task.StoryAttrs.Persona)))
		}
	}

	if task.EpicAttrs != nil && !task.EpicAttrs.IsEmpty() {
		if task.EpicAttrs.Scope != "" {
			parts = append(parts, fmt.Sprintf("epic.scope: %s", task.EpicAttrs.Scope))
		}
		if task.EpicAttrs.Milestone != "" {
			parts = append(parts, fmt.Sprintf("epic.milestone: %s", task.EpicAttrs.Milestone))
		}
		if task.EpicAttrs.SuccessMetrics != "" {
			parts = append(parts, fmt.Sprintf("epic.metrics: %s", escapeNewlines(task.EpicAttrs.SuccessMetrics)))
		}
	}

	// Score dimensions (skip empty keys to prevent malformed metadata)
	if len(task.ScoreDimensions) > 0 {
		// Sort keys for deterministic output
		keys := sortedKeys(task.ScoreDimensions)
		for _, k := range keys {
			if k == "" {
				continue
			}
			parts = append(parts, fmt.Sprintf("score.%s: %d", k, task.ScoreDimensions[k]))
		}
	}

	return fmt.Sprintf("<!-- %s -->", strings.Join(parts, ", "))
}

// FormatRecurrence converts a RecurrencePattern to its string representation for storage.
func FormatRecurrence(r *models.RecurrencePattern) string {
	if r == nil {
		return ""
	}

	// Use compact format: Nd, Nw, Nm, Ny
	var unit string
	switch r.Unit {
	case models.RecurrenceDaily:
		unit = "d"
	case models.RecurrenceWeekly:
		unit = "w"
	case models.RecurrenceMonthly:
		unit = "m"
	case models.RecurrenceYearly:
		unit = "y"
	default:
		unit = "d"
	}

	base := fmt.Sprintf("%d%s", r.Interval, unit)

	// Add weekdays for weekly patterns
	if r.Unit == models.RecurrenceWeekly && len(r.Weekdays) > 0 {
		dayNames := []string{"sun", "mon", "tue", "wed", "thu", "fri", "sat"}
		var days []string
		for _, d := range r.Weekdays {
			if d >= 0 && d < 7 {
				days = append(days, dayNames[d])
			}
		}
		if len(days) > 0 {
			return base + ":" + strings.Join(days, ",")
		}
	}

	return base
}
