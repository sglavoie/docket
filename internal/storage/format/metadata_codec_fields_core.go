package format

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/util"
)

func metadataCoreFieldCodecs() []metadataFieldCodec {
	return []metadataFieldCodec{
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
	}
}
