package format

import (
	"strconv"
	"strings"
	"time"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/util"
)

func metadataCoreFieldCodecs() []metadataFieldCodec {
	return []metadataFieldCodec{
		coreStringField("id",
			func(task *models.Task, value string) { task.ID = value },
			func(task *models.Task) string { return task.ID },
		),
		legacyStatusFieldCodec(),
		metadataStateFieldCodec(),
		metadataPriorityFieldCodec(),
		coreStringPtrField("parent",
			func(task *models.Task, value *string) { task.ParentID = value },
			func(task *models.Task) *string { return task.ParentID },
		),
		coreStringSliceField("tags",
			func(task *models.Task, values []string) { task.Tags = append(task.Tags, values...) },
			func(task *models.Task) []string { return task.Tags },
		),
		legacySectionFieldCodec(),
		coreTimeField("created",
			func(task *models.Task, value time.Time) { task.CreatedAt = value },
			func(task *models.Task) time.Time { return task.CreatedAt },
			func(task *models.Task) bool { return !task.CreatedAt.IsZero() },
			util.DateTimeFormat,
		),
		metadataUpdatedFieldCodec(),
		coreTimePtrField("completed",
			func(task *models.Task, value *time.Time) { task.CompletedAt = value },
			func(task *models.Task) *time.Time { return task.CompletedAt },
			util.DateTimeFormat,
		),
		metadataDueFieldCodec(),
		coreStringSliceField("depends",
			func(task *models.Task, values []string) { task.DependsOn = append(task.DependsOn, values...) },
			func(task *models.Task) []string { return task.DependsOn },
		),
		coreBoolTrueField("pinned",
			func(task *models.Task, value bool) { task.Pinned = value },
			func(task *models.Task) bool { return task.Pinned },
		),
		metadataPinnedAtFieldCodec(),
		legacyActiveFieldCodec(),
		coreDatePtrField("snoozed",
			func(task *models.Task, value *time.Time) { task.SnoozedUntil = value },
			func(task *models.Task) *time.Time { return task.SnoozedUntil },
		),
		metadataRecurFieldCodec(),
		legacyRecurSubField("recur.skipped"),
		legacyRecurSubField("recur.streak"),
		legacyRecurSubField("recur.max_streak"),
		legacyRecurSubField("recur.original_day"),
		metadataAttachmentsFieldCodec(),
		metadataChecklistFieldCodec(),
		legacyStageFieldCodec(),
	}
}

func coreStringField(key string, set func(*models.Task, string), get func(*models.Task) string) metadataFieldCodec {
	return metadataField(key,
		func(task *models.Task, value string, _ *metadataParseState, _ *[]ParseWarning) {
			set(task, value)
		},
		func(task *models.Task, parts *[]string) {
			if value := get(task); value != "" {
				appendMetadataPart(parts, key, value)
			}
		},
	)
}

func coreStringPtrField(key string, set func(*models.Task, *string), get func(*models.Task) *string) metadataFieldCodec {
	return metadataField(key,
		func(task *models.Task, value string, _ *metadataParseState, _ *[]ParseWarning) {
			if value != "" {
				set(task, &value)
			}
		},
		func(task *models.Task, parts *[]string) {
			if value := get(task); value != nil && *value != "" {
				appendMetadataPart(parts, key, *value)
			}
		},
	)
}

func coreStringSliceField(key string, appendValues func(*models.Task, []string), get func(*models.Task) []string) metadataFieldCodec {
	return metadataField(key,
		func(task *models.Task, value string, _ *metadataParseState, _ *[]ParseWarning) {
			appendValues(task, splitSemicolonDedup(value))
		},
		func(task *models.Task, parts *[]string) {
			if values := get(task); len(values) > 0 {
				appendMetadataPart(parts, key, strings.Join(values, ";"))
			}
		},
	)
}

func coreBoolTrueField(key string, set func(*models.Task, bool), get func(*models.Task) bool) metadataFieldCodec {
	return metadataField(key,
		func(task *models.Task, value string, _ *metadataParseState, _ *[]ParseWarning) {
			set(task, value == "true")
		},
		func(task *models.Task, parts *[]string) {
			appendTrueMetadataPart(parts, key, get(task))
		},
	)
}

func coreTimeField(key string, set func(*models.Task, time.Time), get func(*models.Task) time.Time, shouldWrite func(*models.Task) bool, layout string) metadataFieldCodec {
	return metadataField(key,
		func(task *models.Task, value string, _ *metadataParseState, warnings *[]ParseWarning) {
			if parsed, ok := parseTimeOrWarn(value, key, warnings); ok {
				set(task, parsed)
			}
		},
		func(task *models.Task, parts *[]string) {
			if shouldWrite(task) {
				appendMetadataPart(parts, key, get(task).Format(layout))
			}
		},
	)
}

func coreTimePtrField(key string, set func(*models.Task, *time.Time), get func(*models.Task) *time.Time, layout string) metadataFieldCodec {
	return metadataField(key,
		func(task *models.Task, value string, _ *metadataParseState, warnings *[]ParseWarning) {
			if parsed, ok := parseTimeOrWarn(value, key, warnings); ok {
				set(task, &parsed)
			}
		},
		func(task *models.Task, parts *[]string) {
			if value := get(task); value != nil {
				appendMetadataPart(parts, key, value.Format(layout))
			}
		},
	)
}

func coreDatePtrField(key string, set func(*models.Task, *time.Time), get func(*models.Task) *time.Time) metadataFieldCodec {
	return metadataField(key,
		func(task *models.Task, value string, _ *metadataParseState, warnings *[]ParseWarning) {
			if parsed, ok := parseDateOrWarn(value, key, warnings); ok {
				set(task, &parsed)
			}
		},
		func(task *models.Task, parts *[]string) {
			if value := get(task); value != nil {
				appendMetadataPart(parts, key, value.Format(util.DateFormat))
			}
		},
	)
}

func legacyRecurSubField(key string) metadataFieldCodec {
	return metadataField(key,
		func(_ *models.Task, value string, state *metadataParseState, warnings *[]ParseWarning) {
			if _, ok := parseIntOrWarn(value, key, warnings); ok {
				state.legacyRecurrenceSubFieldSeen = true
			}
		},
		func(*models.Task, *[]string) {},
	)
}

func metadataPriorityFieldCodec() metadataFieldCodec {
	return metadataField("priority",
		func(task *models.Task, value string, _ *metadataParseState, warnings *[]ParseWarning) {
			priority, err := strconv.Atoi(value)
			if err != nil {
				addValidationWarning(warnings, "priority", "invalid priority %q, using default", value)
				return
			}
			// Zero round-trips as the canonical unset priority; only values
			// outside the scale fall back to the parser's default.
			if err = models.ValidatePriority(priority, true); err != nil {
				addValidationWarning(warnings, "priority", "priority %d out of range (%d for unset, %d-%d), using default", priority, models.PriorityUnset, models.PriorityMin, models.PriorityMax)
				return
			}
			task.Priority = priority
		},
		func(task *models.Task, parts *[]string) {
			appendMetadataPart(parts, "priority", strconv.Itoa(task.Priority))
		},
	)
}

func metadataUpdatedFieldCodec() metadataFieldCodec {
	return coreTimeField("updated",
		func(task *models.Task, value time.Time) { task.UpdatedAt = value },
		func(task *models.Task) time.Time { return task.UpdatedAt },
		func(task *models.Task) bool {
			// created/updated are persisted at minute precision. Comparing the
			// richer in-memory time values made two timestamps in the same minute
			// serialize as distinct, then parse back as equal; the next canonical
			// render omitted updated and changed the revision. Decide presence in
			// the persisted representation instead so write -> parse -> write is a
			// fixed point.
			return !task.UpdatedAt.IsZero() &&
				task.UpdatedAt.Format(util.DateTimeFormat) != task.CreatedAt.Format(util.DateTimeFormat)
		},
		util.DateTimeFormat,
	)
}

func metadataDueFieldCodec() metadataFieldCodec {
	return metadataField("due",
		func(task *models.Task, value string, _ *metadataParseState, warnings *[]ParseWarning) {
			parseDueOrWarn(task, value, warnings)
		},
		func(task *models.Task, parts *[]string) {
			if task.DueDate == nil {
				return
			}
			appendMetadataPart(parts, "due", task.DueDate.Format(util.DateFormat))
		},
	)
}

func metadataPinnedAtFieldCodec() metadataFieldCodec {
	return metadataField("pinned_at",
		func(task *models.Task, value string, _ *metadataParseState, warnings *[]ParseWarning) {
			if parsed, err := time.ParseInLocation(pinnedAtFormat, value, time.Local); err == nil {
				task.PinnedAt = &parsed
				return
			}
			if parsed, ok := ParseTimeField(value); ok {
				task.PinnedAt = &parsed
				return
			}
			// Emit a warning so a malformed pinned_at doesn't silently drop the
			// user's pin timestamp on the next save (the writer skips PinnedAt
			// when it's nil, so a parse failure here is otherwise invisible).
			addValidationWarning(warnings, "pinned_at", "invalid pinned_at %q, ignoring", value)
		},
		func(task *models.Task, parts *[]string) {
			if task.Pinned && task.PinnedAt != nil {
				appendMetadataPart(parts, "pinned_at", task.PinnedAt.Format(pinnedAtFormat))
			}
		},
	)
}

func metadataRecurFieldCodec() metadataFieldCodec {
	return metadataField("recur",
		func(task *models.Task, value string, _ *metadataParseState, warnings *[]ParseWarning) {
			if pattern := models.ParseRecurrence(value); pattern != nil {
				task.Recurrence = pattern
				return
			}
			addValidationWarning(warnings, "recur", "invalid recurrence %q, ignoring", value)
		},
		func(task *models.Task, parts *[]string) {
			if task.Recurrence != nil {
				if value := FormatRecurrence(task.Recurrence); value != "" {
					appendMetadataPart(parts, "recur", value)
				}
			}
		},
	)
}

func metadataAttachmentsFieldCodec() metadataFieldCodec {
	return metadataField("attachments",
		func(task *models.Task, value string, _ *metadataParseState, warnings *[]ParseWarning) {
			task.Attachments = append(task.Attachments, parseAttachments(value, warnings)...)
		},
		writeAttachmentsMetadata,
	)
}

func metadataChecklistFieldCodec() metadataFieldCodec {
	return metadataField("checklist",
		func(task *models.Task, value string, _ *metadataParseState, _ *[]ParseWarning) {
			parseChecklistMetadata(task, value)
		},
		writeChecklistMetadata,
	)
}
