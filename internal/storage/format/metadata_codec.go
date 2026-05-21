package format

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/util"
)

type metadataParseFunc func(task *models.Task, value string, state *metadataParseState, warnings *[]ParseWarning)
type metadataWriteFunc func(task *models.Task, parts *[]string)
type metadataDynamicParseFunc func(task *models.Task, key, value string, state *metadataParseState, warnings *[]ParseWarning)
type metadataDynamicIndexFunc func(s string) (idx int, key string)

type metadataFieldCodec struct {
	key      string
	taskType models.TaskType
	parse    metadataParseFunc
	write    metadataWriteFunc
}

type metadataFieldFamilyCodec struct {
	recognizes  func(key string) bool
	parseMatch  func(key string) bool
	parse       metadataDynamicParseFunc
	write       metadataWriteFunc
	findSplitAt metadataDynamicIndexFunc
}

type metadataRegistry struct {
	ordered  []metadataFieldCodec
	keys     []string
	byKey    map[string]metadataFieldCodec
	families []metadataFieldFamilyCodec
}

type metadataParseState struct {
	recurSubFields struct {
		skipped     int
		streak      int
		maxStreak   int
		originalDay int
		hasAny      bool
	}
}

func metadataField(key string, parse metadataParseFunc, write metadataWriteFunc) metadataFieldCodec {
	return metadataFieldCodec{
		key:   key,
		parse: parse,
		write: write,
	}
}

func typedMetadataField(key string, taskType models.TaskType, parse metadataParseFunc, write metadataWriteFunc) metadataFieldCodec {
	return metadataFieldCodec{
		key:      key,
		taskType: taskType,
		parse:    parse,
		write:    write,
	}
}

func newMetadataRegistry(fields []metadataFieldCodec, families ...metadataFieldFamilyCodec) metadataRegistry {
	keys := make([]string, 0, len(fields))
	byKey := make(map[string]metadataFieldCodec, len(fields))
	for _, field := range fields {
		keys = append(keys, field.key)
		byKey[field.key] = field
	}

	return metadataRegistry{
		ordered:  fields,
		keys:     keys,
		byKey:    byKey,
		families: families,
	}
}

func (r metadataRegistry) recognizesKey(key string) bool {
	if key == "" {
		return false
	}
	if _, ok := r.byKey[key]; ok {
		return true
	}
	for _, family := range r.families {
		if family.recognizes != nil && family.recognizes(key) {
			return true
		}
	}
	return false
}

func (r metadataRegistry) parse(task *models.Task, key, value string, state *metadataParseState, warnings *[]ParseWarning) bool {
	if field, ok := r.byKey[key]; ok {
		if field.taskType != "" && task.Type != field.taskType {
			*warnings = append(*warnings, ParseWarning{
				Type:    WarningValidation,
				Field:   key,
				Message: fmt.Sprintf("%s attribute on %s task, skipping", key, task.Type),
			})
			return true
		}
		field.parse(task, value, state, warnings)
		return true
	}

	for _, family := range r.families {
		match := family.recognizes
		if family.parseMatch != nil {
			match = family.parseMatch
		}
		if match != nil && match(key) {
			family.parse(task, key, value, state, warnings)
			return true
		}
	}

	return false
}

func (r metadataRegistry) appendWrittenParts(task *models.Task, parts *[]string) {
	for _, field := range r.ordered {
		field.write(task, parts)
	}
	for _, family := range r.families {
		family.write(task, parts)
	}
}

func (r metadataRegistry) nextSplitIndex(remaining string, seenKeys map[string]bool) int {
	splitIdx := -1

	for _, key := range r.keys {
		if seenKeys[key] {
			continue
		}
		pattern := ", " + key + ":"
		idx := strings.Index(remaining, pattern)
		if idx != -1 && (splitIdx == -1 || idx < splitIdx) {
			splitIdx = idx
		}
	}

	for _, family := range r.families {
		if family.findSplitAt == nil {
			continue
		}
		idx, key := family.findSplitAt(remaining)
		if idx != -1 && !seenKeys[key] && (splitIdx == -1 || idx < splitIdx) {
			splitIdx = idx
		}
	}

	return splitIdx
}

func appendMetadataPart(parts *[]string, key, value string) {
	*parts = append(*parts, fmt.Sprintf("%s: %s", key, value))
}

func appendTrueMetadataPart(parts *[]string, key string, value bool) {
	if value {
		appendMetadataPart(parts, key, "true")
	}
}

func parseChecklistMetadata(task *models.Task, value string) {
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
			text = strings.TrimSuffix(item, "[ ]")
		}
		text = unescapeChecklistText(strings.TrimSpace(text))
		if text != "" {
			task.Checklist = append(task.Checklist, models.ChecklistItem{Text: text, Done: done})
		}
	}
}

func writeAttachmentsMetadata(task *models.Task, parts *[]string) {
	if len(task.Attachments) == 0 {
		return
	}

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
		if att.OriginalPath != "" {
			if att.IsCodeRef && att.LineNumber > 0 {
				attStr += fmt.Sprintf("||%s:%d", att.OriginalPath, att.LineNumber)
			} else {
				attStr += "||" + att.OriginalPath
			}
		}
		attStrings = append(attStrings, attStr+fmt.Sprintf("@%d", att.AddedAt.Unix()))
	}

	appendMetadataPart(parts, "attachments", strings.Join(attStrings, ";"))
}

func writeChecklistMetadata(task *models.Task, parts *[]string) {
	if len(task.Checklist) == 0 {
		return
	}

	var checklistStrings []string
	for _, item := range task.Checklist {
		marker := "[ ]"
		if item.Done {
			marker = "[x]"
		}
		checklistStrings = append(checklistStrings, escapeChecklistText(item.Text)+marker)
	}

	appendMetadataPart(parts, "checklist", strings.Join(checklistStrings, ";"))
}

func findScoreMetadataSplitIndex(s string) (int, string) {
	idx := findScoreKeyIndex(s)
	if idx == -1 {
		return -1, ""
	}
	return idx, extractScoreKey(s[idx+2:])
}

func parseScoreMetadata(task *models.Task, key, value string, warnings *[]ParseWarning) {
	dimID := strings.TrimPrefix(key, "score.")
	if !isValidDimensionID(dimID) {
		*warnings = append(*warnings, ParseWarning{
			Type:    WarningValidation,
			Field:   key,
			Message: fmt.Sprintf("invalid score dimension ID '%s' (must be 1-%d alphanumeric/underscore chars), skipping", dimID, MaxDimensionIDLength),
		})
		return
	}

	v, err := strconv.Atoi(value)
	if err != nil {
		*warnings = append(*warnings, ParseWarning{
			Type:    WarningValidation,
			Field:   key,
			Message: fmt.Sprintf("invalid score value '%s' for dimension '%s', skipping", value, dimID),
		})
		return
	}

	if task.ScoreDimensions == nil {
		task.ScoreDimensions = make(map[string]int)
	}
	task.ScoreDimensions[dimID] = v
}

func writeScoreMetadata(task *models.Task, parts *[]string) {
	if len(task.ScoreDimensions) == 0 {
		return
	}

	for _, key := range sortedKeys(task.ScoreDimensions) {
		if key == "" || !isValidDimensionID(key) {
			continue
		}
		appendMetadataPart(parts, "score."+key, strconv.Itoa(task.ScoreDimensions[key]))
	}
}

// metadataFieldRegistry is the single source of truth for metadata key
// recognition, parsing, and serialization order.
var metadataFieldRegistry = newMetadataRegistry(
	[]metadataFieldCodec{
		metadataField("id",
			func(task *models.Task, value string, _ *metadataParseState, _ *[]ParseWarning) {
				task.ID = value
			},
			func(task *models.Task, parts *[]string) {
				if task.ID != "" {
					appendMetadataPart(parts, "id", task.ID)
				}
			},
		),
		metadataField("status",
			func(task *models.Task, value string, _ *metadataParseState, warnings *[]ParseWarning) {
				if status, ok := models.ParseTaskStatus(value); ok {
					task.Status = status
					return
				}
				addValidationWarning(warnings, "status", "invalid status %q, using default", value)
			},
			func(task *models.Task, parts *[]string) {
				appendMetadataPart(parts, "status", string(task.Status))
			},
		),
		metadataField("priority",
			func(task *models.Task, value string, _ *metadataParseState, warnings *[]ParseWarning) {
				p, err := strconv.Atoi(value)
				if err != nil {
					addValidationWarning(warnings, "priority", "invalid priority %q, using default", value)
					return
				}
				if err := models.ValidatePriority(p, false); err != nil {
					addValidationWarning(warnings, "priority", "priority %d out of range (%d-%d), using default", p, models.PriorityMin, models.PriorityMax)
					return
				}
				task.Priority = p
			},
			func(task *models.Task, parts *[]string) {
				appendMetadataPart(parts, "priority", strconv.Itoa(task.Priority))
			},
		),
		metadataField("parent",
			func(task *models.Task, value string, _ *metadataParseState, _ *[]ParseWarning) {
				if value != "" {
					task.ParentID = &value
				}
			},
			func(task *models.Task, parts *[]string) {
				if task.ParentID != nil && *task.ParentID != "" {
					appendMetadataPart(parts, "parent", *task.ParentID)
				}
			},
		),
		metadataField("tags",
			func(task *models.Task, value string, _ *metadataParseState, _ *[]ParseWarning) {
				task.Tags = append(task.Tags, splitSemicolonDedup(value)...)
			},
			func(task *models.Task, parts *[]string) {
				if len(task.Tags) > 0 {
					appendMetadataPart(parts, "tags", strings.Join(task.Tags, ";"))
				}
			},
		),
		metadataField("section",
			func(task *models.Task, value string, _ *metadataParseState, _ *[]ParseWarning) {
				task.Section = value
			},
			func(task *models.Task, parts *[]string) {
				if task.Section != "" {
					appendMetadataPart(parts, "section", task.Section)
				}
			},
		),
		metadataField("custom_section",
			func(task *models.Task, value string, _ *metadataParseState, _ *[]ParseWarning) {
				task.CustomSection = value
			},
			func(task *models.Task, parts *[]string) {
				if task.CustomSection != "" {
					appendMetadataPart(parts, "custom_section", task.CustomSection)
				}
			},
		),
		metadataField("created",
			func(task *models.Task, value string, _ *metadataParseState, warnings *[]ParseWarning) {
				if t, ok := parseTimeOrWarn(value, "created", warnings); ok {
					task.CreatedAt = t
				}
			},
			func(task *models.Task, parts *[]string) {
				if !task.CreatedAt.IsZero() {
					appendMetadataPart(parts, "created", task.CreatedAt.Format(util.DateTimeFormat))
				}
			},
		),
		metadataField("updated",
			func(task *models.Task, value string, _ *metadataParseState, warnings *[]ParseWarning) {
				if t, ok := parseTimeOrWarn(value, "updated", warnings); ok {
					task.UpdatedAt = t
				}
			},
			func(task *models.Task, parts *[]string) {
				if !task.UpdatedAt.IsZero() && !task.UpdatedAt.Equal(task.CreatedAt) {
					appendMetadataPart(parts, "updated", task.UpdatedAt.Format(util.DateTimeFormat))
				}
			},
		),
		metadataField("completed",
			func(task *models.Task, value string, _ *metadataParseState, warnings *[]ParseWarning) {
				if t, ok := parseTimeOrWarn(value, "completed", warnings); ok {
					task.CompletedAt = &t
				}
			},
			func(task *models.Task, parts *[]string) {
				if task.CompletedAt != nil {
					appendMetadataPart(parts, "completed", task.CompletedAt.Format(util.DateTimeFormat))
				}
			},
		),
		metadataField("due",
			func(task *models.Task, value string, _ *metadataParseState, warnings *[]ParseWarning) {
				parseDueOrWarn(task, value, warnings)
			},
			func(task *models.Task, parts *[]string) {
				if task.DueDate == nil {
					return
				}
				if task.HasTime {
					appendMetadataPart(parts, "due", task.DueDate.Format(util.DateTimeDueFormat))
					return
				}
				appendMetadataPart(parts, "due", task.DueDate.Format(util.DateFormat))
			},
		),
		metadataField("start",
			func(task *models.Task, value string, _ *metadataParseState, warnings *[]ParseWarning) {
				if t, ok := parseDateOrWarn(value, "start", warnings); ok {
					task.StartDate = &t
				}
			},
			func(task *models.Task, parts *[]string) {
				if task.StartDate != nil {
					appendMetadataPart(parts, "start", task.StartDate.Format(util.DateFormat))
				}
			},
		),
		metadataField("estimated_days",
			func(task *models.Task, value string, _ *metadataParseState, warnings *[]ParseWarning) {
				if v, ok := parseFloatOrWarn(value, "estimated_days", warnings); ok && v > 0 {
					task.EstimatedDays = v
				}
			},
			func(task *models.Task, parts *[]string) {
				if task.EstimatedDays > 0 {
					appendMetadataPart(parts, "estimated_days", fmt.Sprintf("%g", task.EstimatedDays))
				}
			},
		),
		metadataField("depends",
			func(task *models.Task, value string, _ *metadataParseState, _ *[]ParseWarning) {
				task.DependsOn = append(task.DependsOn, splitSemicolonDedup(value)...)
			},
			func(task *models.Task, parts *[]string) {
				if len(task.DependsOn) > 0 {
					appendMetadataPart(parts, "depends", strings.Join(task.DependsOn, ";"))
				}
			},
		),
		metadataField("related",
			func(task *models.Task, value string, _ *metadataParseState, _ *[]ParseWarning) {
				task.RelatedTo = append(task.RelatedTo, splitSemicolonDedup(value)...)
			},
			func(task *models.Task, parts *[]string) {
				if len(task.RelatedTo) > 0 {
					appendMetadataPart(parts, "related", strings.Join(task.RelatedTo, ";"))
				}
			},
		),
		metadataField("pinned",
			func(task *models.Task, value string, _ *metadataParseState, _ *[]ParseWarning) {
				task.Pinned = value == "true"
			},
			func(task *models.Task, parts *[]string) {
				appendTrueMetadataPart(parts, "pinned", task.Pinned)
			},
		),
		metadataField("pinned_at",
			func(task *models.Task, value string, _ *metadataParseState, _ *[]ParseWarning) {
				if t, err := time.ParseInLocation(pinnedAtFormat, value, time.Local); err == nil {
					task.PinnedAt = &t
					return
				}
				if t, ok := ParseTimeField(value); ok {
					task.PinnedAt = &t
				}
			},
			func(task *models.Task, parts *[]string) {
				if task.Pinned && task.PinnedAt != nil {
					appendMetadataPart(parts, "pinned_at", task.PinnedAt.Format(pinnedAtFormat))
				}
			},
		),
		metadataField("active",
			func(task *models.Task, value string, _ *metadataParseState, _ *[]ParseWarning) {
				task.Active = value == "true"
			},
			func(task *models.Task, parts *[]string) {
				appendTrueMetadataPart(parts, "active", task.Active)
			},
		),
		metadataField("dimmed",
			func(task *models.Task, value string, _ *metadataParseState, _ *[]ParseWarning) {
				task.Dimmed = value == "true"
			},
			func(task *models.Task, parts *[]string) {
				appendTrueMetadataPart(parts, "dimmed", task.Dimmed)
			},
		),
		metadataField("snoozed",
			func(task *models.Task, value string, _ *metadataParseState, warnings *[]ParseWarning) {
				if t, ok := parseDateOrWarn(value, "snoozed", warnings); ok {
					task.SnoozedUntil = &t
				}
			},
			func(task *models.Task, parts *[]string) {
				if task.SnoozedUntil != nil {
					appendMetadataPart(parts, "snoozed", task.SnoozedUntil.Format(util.DateFormat))
				}
			},
		),
		metadataField("recur",
			func(task *models.Task, value string, _ *metadataParseState, warnings *[]ParseWarning) {
				if pattern := models.ParseRecurrence(value); pattern != nil {
					task.Recurrence = pattern
					return
				}
				addValidationWarning(warnings, "recur", "invalid recurrence %q, ignoring", value)
			},
			func(task *models.Task, parts *[]string) {
				if task.Recurrence != nil {
					appendMetadataPart(parts, "recur", FormatRecurrence(task.Recurrence))
				}
			},
		),
		metadataField("recur.skipped",
			func(_ *models.Task, value string, state *metadataParseState, warnings *[]ParseWarning) {
				if v, ok := parseIntOrWarn(value, "recur.skipped", warnings); ok {
					state.recurSubFields.skipped = v
					state.recurSubFields.hasAny = true
				}
			},
			func(task *models.Task, parts *[]string) {
				if task.Recurrence != nil && task.Recurrence.Skipped > 0 {
					appendMetadataPart(parts, "recur.skipped", strconv.Itoa(task.Recurrence.Skipped))
				}
			},
		),
		metadataField("recur.streak",
			func(_ *models.Task, value string, state *metadataParseState, warnings *[]ParseWarning) {
				if v, ok := parseIntOrWarn(value, "recur.streak", warnings); ok {
					state.recurSubFields.streak = v
					state.recurSubFields.hasAny = true
				}
			},
			func(task *models.Task, parts *[]string) {
				if task.Recurrence != nil && task.Recurrence.Streak > 0 {
					appendMetadataPart(parts, "recur.streak", strconv.Itoa(task.Recurrence.Streak))
				}
			},
		),
		metadataField("recur.max_streak",
			func(_ *models.Task, value string, state *metadataParseState, warnings *[]ParseWarning) {
				if v, ok := parseIntOrWarn(value, "recur.max_streak", warnings); ok {
					state.recurSubFields.maxStreak = v
					state.recurSubFields.hasAny = true
				}
			},
			func(task *models.Task, parts *[]string) {
				if task.Recurrence != nil && task.Recurrence.MaxStreak > 0 {
					appendMetadataPart(parts, "recur.max_streak", strconv.Itoa(task.Recurrence.MaxStreak))
				}
			},
		),
		metadataField("recur.original_day",
			func(_ *models.Task, value string, state *metadataParseState, warnings *[]ParseWarning) {
				if v, ok := parseIntOrWarn(value, "recur.original_day", warnings); ok {
					state.recurSubFields.originalDay = v
					state.recurSubFields.hasAny = true
				}
			},
			func(task *models.Task, parts *[]string) {
				if task.Recurrence != nil && task.Recurrence.OriginalDay > 0 {
					appendMetadataPart(parts, "recur.original_day", strconv.Itoa(task.Recurrence.OriginalDay))
				}
			},
		),
		metadataField("attachments",
			func(task *models.Task, value string, _ *metadataParseState, warnings *[]ParseWarning) {
				task.Attachments = append(task.Attachments, parseAttachments(value, warnings)...)
			},
			writeAttachmentsMetadata,
		),
		metadataField("checklist",
			func(task *models.Task, value string, _ *metadataParseState, _ *[]ParseWarning) {
				parseChecklistMetadata(task, value)
			},
			writeChecklistMetadata,
		),
		metadataField("stage",
			func(task *models.Task, value string, _ *metadataParseState, _ *[]ParseWarning) {
				task.Stage = value
			},
			func(task *models.Task, parts *[]string) {
				if task.Stage != "" {
					appendMetadataPart(parts, "stage", task.Stage)
				}
			},
		),
		metadataField("source",
			func(task *models.Task, value string, _ *metadataParseState, _ *[]ParseWarning) {
				task.Source = value
			},
			func(task *models.Task, parts *[]string) {
				if task.Source != "" {
					appendMetadataPart(parts, "source", task.Source)
				}
			},
		),
		metadataField("url",
			func(task *models.Task, value string, _ *metadataParseState, _ *[]ParseWarning) {
				task.URL = value
			},
			func(task *models.Task, parts *[]string) {
				if task.URL != "" {
					appendMetadataPart(parts, "url", task.URL)
				}
			},
		),
		typedMetadataField("bug.reproducible", models.TaskTypeBug,
			func(task *models.Task, value string, _ *metadataParseState, warnings *[]ParseWarning) {
				reproducible := models.BugReproducible(value)
				if !reproducible.IsValid() {
					addValidationWarning(warnings, "bug.reproducible", "invalid bug.reproducible %q, ignoring", value)
					return
				}
				task.EnsureBugAttrs().Reproducible = reproducible
			},
			func(task *models.Task, parts *[]string) {
				if task.BugAttrs != nil && !task.BugAttrs.IsEmpty() && task.BugAttrs.Reproducible != "" {
					appendMetadataPart(parts, "bug.reproducible", string(task.BugAttrs.Reproducible))
				}
			},
		),
		typedMetadataField("bug.severity", models.TaskTypeBug,
			func(task *models.Task, value string, _ *metadataParseState, warnings *[]ParseWarning) {
				severity := models.BugSeverity(value)
				if !severity.IsValid() {
					addValidationWarning(warnings, "bug.severity", "invalid bug.severity %q, ignoring", value)
					return
				}
				task.EnsureBugAttrs().Severity = severity
			},
			func(task *models.Task, parts *[]string) {
				if task.BugAttrs != nil && !task.BugAttrs.IsEmpty() && task.BugAttrs.Severity != "" {
					appendMetadataPart(parts, "bug.severity", string(task.BugAttrs.Severity))
				}
			},
		),
		typedMetadataField("bug.workaround", models.TaskTypeBug,
			func(task *models.Task, value string, _ *metadataParseState, _ *[]ParseWarning) {
				task.EnsureBugAttrs().Workaround = unescapeNewlines(value)
			},
			func(task *models.Task, parts *[]string) {
				if task.BugAttrs != nil && !task.BugAttrs.IsEmpty() && task.BugAttrs.Workaround != "" {
					appendMetadataPart(parts, "bug.workaround", escapeNewlines(task.BugAttrs.Workaround))
				}
			},
		),
		typedMetadataField("idea.category", models.TaskTypeIdea,
			func(task *models.Task, value string, _ *metadataParseState, warnings *[]ParseWarning) {
				category := models.IdeaCategory(value)
				if !category.IsValid() {
					addValidationWarning(warnings, "idea.category", "invalid idea.category %q, ignoring", value)
					return
				}
				task.EnsureIdeaAttrs().Category = category
			},
			func(task *models.Task, parts *[]string) {
				if task.IdeaAttrs != nil && !task.IdeaAttrs.IsEmpty() && task.IdeaAttrs.Category != "" {
					appendMetadataPart(parts, "idea.category", string(task.IdeaAttrs.Category))
				}
			},
		),
		typedMetadataField("idea.effort", models.TaskTypeIdea,
			func(task *models.Task, value string, _ *metadataParseState, warnings *[]ParseWarning) {
				effort := models.EffortEstimate(value)
				if !effort.IsValid() {
					addValidationWarning(warnings, "idea.effort", "invalid idea.effort %q, ignoring", value)
					return
				}
				task.EnsureIdeaAttrs().Effort = effort
			},
			func(task *models.Task, parts *[]string) {
				if task.IdeaAttrs != nil && !task.IdeaAttrs.IsEmpty() && task.IdeaAttrs.Effort != "" {
					appendMetadataPart(parts, "idea.effort", string(task.IdeaAttrs.Effort))
				}
			},
		),
		typedMetadataField("idea.value", models.TaskTypeIdea,
			func(task *models.Task, value string, _ *metadataParseState, warnings *[]ParseWarning) {
				ideaValue := models.IdeaValue(value)
				if !ideaValue.IsValid() {
					addValidationWarning(warnings, "idea.value", "invalid idea.value %q, ignoring", value)
					return
				}
				task.EnsureIdeaAttrs().Value = ideaValue
			},
			func(task *models.Task, parts *[]string) {
				if task.IdeaAttrs != nil && !task.IdeaAttrs.IsEmpty() && task.IdeaAttrs.Value != "" {
					appendMetadataPart(parts, "idea.value", string(task.IdeaAttrs.Value))
				}
			},
		),
		typedMetadataField("idea.validated", models.TaskTypeIdea,
			func(task *models.Task, value string, _ *metadataParseState, _ *[]ParseWarning) {
				task.EnsureIdeaAttrs().Validated = value == "true"
			},
			func(task *models.Task, parts *[]string) {
				if task.IdeaAttrs != nil && !task.IdeaAttrs.IsEmpty() && task.IdeaAttrs.Validated {
					appendMetadataPart(parts, "idea.validated", "true")
				}
			},
		),
		typedMetadataField("task.effort", models.TaskTypeTask,
			func(task *models.Task, value string, _ *metadataParseState, warnings *[]ParseWarning) {
				effort := models.EffortEstimate(value)
				if !effort.IsValid() {
					addValidationWarning(warnings, "task.effort", "invalid task.effort %q, ignoring", value)
					return
				}
				task.EnsureTaskAttrs().Effort = effort
			},
			func(task *models.Task, parts *[]string) {
				if task.TaskAttrs != nil && !task.TaskAttrs.IsEmpty() && task.TaskAttrs.Effort != "" {
					appendMetadataPart(parts, "task.effort", string(task.TaskAttrs.Effort))
				}
			},
		),
		typedMetadataField("task.worktype", models.TaskTypeTask,
			func(task *models.Task, value string, _ *metadataParseState, warnings *[]ParseWarning) {
				workType := models.WorkType(value)
				if !workType.IsValid() {
					addValidationWarning(warnings, "task.worktype", "invalid task.worktype %q, ignoring", value)
					return
				}
				task.EnsureTaskAttrs().WorkType = workType
			},
			func(task *models.Task, parts *[]string) {
				if task.TaskAttrs != nil && !task.TaskAttrs.IsEmpty() && task.TaskAttrs.WorkType != "" {
					appendMetadataPart(parts, "task.worktype", string(task.TaskAttrs.WorkType))
				}
			},
		),
		typedMetadataField("story.points", models.TaskTypeStory,
			func(task *models.Task, value string, _ *metadataParseState, warnings *[]ParseWarning) {
				if points, ok := parseIntOrWarn(value, "story.points", warnings); ok {
					if points < 0 {
						addValidationWarning(warnings, "story.points", "invalid story.points %d (must be non-negative), ignoring", points)
						return
					}
					task.EnsureStoryAttrs().StoryPoints = points
				}
			},
			func(task *models.Task, parts *[]string) {
				if task.StoryAttrs != nil && !task.StoryAttrs.IsEmpty() && task.StoryAttrs.StoryPoints > 0 {
					appendMetadataPart(parts, "story.points", strconv.Itoa(task.StoryAttrs.StoryPoints))
				}
			},
		),
		typedMetadataField("story.persona", models.TaskTypeStory,
			func(task *models.Task, value string, _ *metadataParseState, _ *[]ParseWarning) {
				task.EnsureStoryAttrs().Persona = unescapeNewlines(value)
			},
			func(task *models.Task, parts *[]string) {
				if task.StoryAttrs != nil && !task.StoryAttrs.IsEmpty() && task.StoryAttrs.Persona != "" {
					appendMetadataPart(parts, "story.persona", escapeNewlines(task.StoryAttrs.Persona))
				}
			},
		),
		typedMetadataField("epic.scope", models.TaskTypeEpic,
			func(task *models.Task, value string, _ *metadataParseState, warnings *[]ParseWarning) {
				scope := models.EpicScope(value)
				if !scope.IsValid() {
					addValidationWarning(warnings, "epic.scope", "invalid epic.scope %q, ignoring", value)
					return
				}
				task.EnsureEpicAttrs().Scope = scope
			},
			func(task *models.Task, parts *[]string) {
				if task.EpicAttrs != nil && !task.EpicAttrs.IsEmpty() && task.EpicAttrs.Scope != "" {
					appendMetadataPart(parts, "epic.scope", string(task.EpicAttrs.Scope))
				}
			},
		),
		typedMetadataField("epic.milestone", models.TaskTypeEpic,
			func(task *models.Task, value string, _ *metadataParseState, _ *[]ParseWarning) {
				task.EnsureEpicAttrs().Milestone = unescapeNewlines(value)
			},
			func(task *models.Task, parts *[]string) {
				if task.EpicAttrs != nil && !task.EpicAttrs.IsEmpty() && task.EpicAttrs.Milestone != "" {
					appendMetadataPart(parts, "epic.milestone", escapeNewlines(task.EpicAttrs.Milestone))
				}
			},
		),
		typedMetadataField("epic.metrics", models.TaskTypeEpic,
			func(task *models.Task, value string, _ *metadataParseState, _ *[]ParseWarning) {
				task.EnsureEpicAttrs().SuccessMetrics = unescapeNewlines(value)
			},
			func(task *models.Task, parts *[]string) {
				if task.EpicAttrs != nil && !task.EpicAttrs.IsEmpty() && task.EpicAttrs.SuccessMetrics != "" {
					appendMetadataPart(parts, "epic.metrics", escapeNewlines(task.EpicAttrs.SuccessMetrics))
				}
			},
		),
		typedMetadataField("epic.initial_children", models.TaskTypeEpic,
			func(task *models.Task, value string, _ *metadataParseState, warnings *[]ParseWarning) {
				if v, ok := parseIntOrWarn(value, "epic.initial_children", warnings); ok && v > 0 {
					task.EnsureEpicAttrs().InitialChildCount = v
				}
			},
			func(task *models.Task, parts *[]string) {
				if task.EpicAttrs != nil && !task.EpicAttrs.IsEmpty() && task.EpicAttrs.InitialChildCount > 0 {
					appendMetadataPart(parts, "epic.initial_children", strconv.Itoa(task.EpicAttrs.InitialChildCount))
				}
			},
		),
		typedMetadataField("epic.scope_baselined", models.TaskTypeEpic,
			func(task *models.Task, value string, _ *metadataParseState, warnings *[]ParseWarning) {
				if t, ok := parseDateOrWarn(value, "epic.scope_baselined", warnings); ok {
					task.EnsureEpicAttrs().ScopeBaselinedAt = &t
				}
			},
			func(task *models.Task, parts *[]string) {
				if task.EpicAttrs != nil && !task.EpicAttrs.IsEmpty() && task.EpicAttrs.ScopeBaselinedAt != nil {
					appendMetadataPart(parts, "epic.scope_baselined", task.EpicAttrs.ScopeBaselinedAt.Format(util.DateFormat))
				}
			},
		),
	},
	metadataFieldFamilyCodec{
		recognizes: func(key string) bool {
			return strings.HasPrefix(key, "score.") && isValidDimensionID(strings.TrimPrefix(key, "score."))
		},
		parseMatch: func(key string) bool {
			return strings.HasPrefix(key, "score.")
		},
		parse: func(task *models.Task, key, value string, _ *metadataParseState, warnings *[]ParseWarning) {
			parseScoreMetadata(task, key, value, warnings)
		},
		write:       writeScoreMetadata,
		findSplitAt: findScoreMetadataSplitIndex,
	},
)
