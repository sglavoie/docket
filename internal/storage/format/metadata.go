package format

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/util"
)

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
	"completed", "due", "depends", "related", "section", "custom_section", "pinned", "active", "recur",
	"attachments", "snoozed", "checklist",
	// Type-specific attribute keys
	"bug.reproducible", "bug.severity", "bug.workaround",
	"idea.category", "idea.effort", "idea.value", "idea.validated",
	"task.effort", "task.worktype",
	"story.points", "story.persona",
	"epic.scope", "epic.milestone", "epic.metrics",
}

// ParseMetadata parses the metadata string and sets task fields.
// Returns any warnings encountered during parsing.
func ParseMetadata(task *models.Task, metaStr string) []ParseWarning {
	var warnings []ParseWarning

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
			task.ParentID = &value
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
		case "active":
			task.Active = value == "true"
		case "recur":
			if pattern := models.ParseRecurrence(value); pattern != nil {
				task.Recurrence = pattern
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

				// Then, extract original path from the timestamp-stripped string
				// For code refs, format is: code:filename||originalPath:lineNumber
				if origIdx := strings.Index(att, "||"); origIdx > 0 {
					originalPath = att[origIdx+2:]
					att = att[:origIdx]
				}

				if strings.HasPrefix(att, "url:") {
					filename := strings.TrimPrefix(att, "url:")
					if originalPath == "" {
						originalPath = filename // URLs: original path same as filename if not specified
					}
					task.Attachments = append(task.Attachments, models.Attachment{
						Filename:     filename,
						IsURL:        true,
						OriginalPath: originalPath,
						AddedAt:      addedAt,
					})
				} else if strings.HasPrefix(att, "code:") {
					filename := strings.TrimPrefix(att, "code:")
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
			items := strings.Split(value, ";")
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
				text = strings.TrimSpace(text)
				if text != "" {
					task.Checklist = append(task.Checklist, models.ChecklistItem{
						Text: text,
						Done: done,
					})
				}
			}

		// Bug attributes
		case "bug.reproducible":
			if task.BugAttrs == nil {
				task.BugAttrs = &models.BugAttrs{}
			}
			task.BugAttrs.Reproducible = models.BugReproducible(value)
		case "bug.severity":
			if task.BugAttrs == nil {
				task.BugAttrs = &models.BugAttrs{}
			}
			task.BugAttrs.Severity = models.BugSeverity(value)
		case "bug.workaround":
			if task.BugAttrs == nil {
				task.BugAttrs = &models.BugAttrs{}
			}
			task.BugAttrs.Workaround = unescapeNewlines(value)

		// Idea attributes
		case "idea.category":
			if task.IdeaAttrs == nil {
				task.IdeaAttrs = &models.IdeaAttrs{}
			}
			task.IdeaAttrs.Category = models.IdeaCategory(value)
		case "idea.effort":
			if task.IdeaAttrs == nil {
				task.IdeaAttrs = &models.IdeaAttrs{}
			}
			task.IdeaAttrs.Effort = models.EffortEstimate(value)
		case "idea.value":
			if task.IdeaAttrs == nil {
				task.IdeaAttrs = &models.IdeaAttrs{}
			}
			task.IdeaAttrs.Value = models.IdeaValue(value)
		case "idea.validated":
			if task.IdeaAttrs == nil {
				task.IdeaAttrs = &models.IdeaAttrs{}
			}
			task.IdeaAttrs.Validated = value == "true"

		// Task attributes
		case "task.effort":
			if task.TaskAttrs == nil {
				task.TaskAttrs = &models.TaskAttrs{}
			}
			task.TaskAttrs.Effort = models.EffortEstimate(value)
		case "task.worktype":
			if task.TaskAttrs == nil {
				task.TaskAttrs = &models.TaskAttrs{}
			}
			task.TaskAttrs.WorkType = models.WorkType(value)

		// Story attributes
		case "story.points":
			if task.StoryAttrs == nil {
				task.StoryAttrs = &models.StoryAttrs{}
			}
			if points, err := strconv.Atoi(value); err == nil {
				task.StoryAttrs.StoryPoints = points
			}
		case "story.persona":
			if task.StoryAttrs == nil {
				task.StoryAttrs = &models.StoryAttrs{}
			}
			task.StoryAttrs.Persona = unescapeNewlines(value)

		// Epic attributes
		case "epic.scope":
			if task.EpicAttrs == nil {
				task.EpicAttrs = &models.EpicAttrs{}
			}
			task.EpicAttrs.Scope = models.EpicScope(value)
		case "epic.milestone":
			if task.EpicAttrs == nil {
				task.EpicAttrs = &models.EpicAttrs{}
			}
			task.EpicAttrs.Milestone = value
		case "epic.metrics":
			if task.EpicAttrs == nil {
				task.EpicAttrs = &models.EpicAttrs{}
			}
			task.EpicAttrs.SuccessMetrics = unescapeNewlines(value)
		}
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
// It looks for ", key:" patterns where key is a known metadata key.
func splitMetadataPairs(metaStr string) []string {
	var pairs []string
	remaining := metaStr

	for len(remaining) > 0 {
		// Find the earliest occurrence of ", knownKey:" in the remaining string
		splitIdx := -1
		for _, key := range metadataKeys {
			pattern := ", " + key + ":"
			idx := strings.Index(remaining, pattern)
			if idx != -1 && (splitIdx == -1 || idx < splitIdx) {
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

// WriteMetadata generates the metadata comment for a task.
func WriteMetadata(task *models.Task) string {
	var parts []string

	if task.ID != "" {
		parts = append(parts, fmt.Sprintf("id: %s", task.ID))
	}

	parts = append(parts, fmt.Sprintf("status: %s", task.Status))
	parts = append(parts, fmt.Sprintf("priority: %d", task.Priority))

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

	if task.Active {
		parts = append(parts, "active: true")
	}

	if task.SnoozedUntil != nil {
		parts = append(parts, fmt.Sprintf("snoozed: %s", task.SnoozedUntil.Format(util.DateFormat)))
	}

	if task.Recurrence != nil {
		parts = append(parts, fmt.Sprintf("recur: %s", FormatRecurrence(task.Recurrence)))
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
			checklistStrings = append(checklistStrings, item.Text+marker)
		}
		parts = append(parts, fmt.Sprintf("checklist: %s", strings.Join(checklistStrings, ";")))
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
