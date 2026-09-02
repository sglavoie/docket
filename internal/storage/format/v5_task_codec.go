package format

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// This file owns the v5 metadata comment: the one line that carries a task's
// scalar record. Record validation lives in v5_task_validate.go, the compound
// field encodings in v5_task_fields.go, and the surrounding rich-text blocks in
// v5_task_body.go.

const (
	v5DateFormat            = "2006-01-02"
	v5MaxRecurrenceInterval = 3650
)

var v5MetadataKeys = map[string]bool{
	"id": true, "short_id": true, "state": true, "priority": true,
	"parent": true, "tags": true, "created": true, "updated": true,
	"completed": true, "due": true, "snoozed": true, "depends": true,
	"pinned": true, "pinned_at": true, "recur": true,
	"attachments": true, "checklist": true,
}

// WriteV5TaskMetadata writes only target v5 fields in one deterministic
// metadata comment. Compound values are query-escaped so delimiters cannot
// corrupt the record boundary.
func WriteV5TaskMetadata(task V5TaskRecord) (string, error) {
	if err := validateV5TaskRecord(task, false); err != nil {
		return "", err
	}
	parts := []string{
		"id: " + task.ID,
		"short_id: " + strconv.Itoa(task.ShortID),
		"state: " + task.State,
		"priority: " + strconv.Itoa(task.Priority),
	}
	if task.ParentID != "" {
		parts = append(parts, "parent: "+task.ParentID)
	}
	if len(task.Tags) > 0 {
		parts = append(parts, "tags: "+strings.Join(task.Tags, ";"))
	}
	parts = append(parts,
		"created: "+formatV5Time(task.CreatedAt),
		"updated: "+formatV5Time(task.UpdatedAt),
	)
	if task.CompletedAt != nil {
		parts = append(parts, "completed: "+formatV5Time(*task.CompletedAt))
	}
	if task.DueDate != "" {
		parts = append(parts, "due: "+task.DueDate)
	}
	if task.SnoozedUntil != "" {
		parts = append(parts, "snoozed: "+task.SnoozedUntil)
	}
	if len(task.DependsOn) > 0 {
		parts = append(parts, "depends: "+strings.Join(task.DependsOn, ";"))
	}
	if task.Pinned {
		parts = append(parts, "pinned: true")
	}
	if task.PinnedAt != nil {
		parts = append(parts, "pinned_at: "+formatV5Time(*task.PinnedAt))
	}
	if task.Recurrence != nil {
		parts = append(parts, "recur: "+formatV5Recurrence(*task.Recurrence))
	}
	if len(task.Attachments) > 0 {
		parts = append(parts, "attachments: "+encodeV5Attachments(task.Attachments))
	}
	if len(task.Checklist) > 0 {
		parts = append(parts, "checklist: "+encodeV5Checklist(task.Checklist))
	}
	return "<!-- " + strings.Join(parts, ", ") + " -->", nil
}

// ParseV5TaskMetadata strictly parses a canonical v5 metadata comment. It does
// not recognize legacy fields, task types, or catalog metadata.
func ParseV5TaskMetadata(comment string) (V5TaskRecord, error) {
	match := MetadataRegex().FindStringSubmatch(strings.TrimSpace(comment))
	if match == nil {
		return V5TaskRecord{}, fmt.Errorf("invalid v5 task metadata comment")
	}
	values := make(map[string]string)
	for _, pair := range strings.Split(match[1], ", ") {
		key, value, ok := strings.Cut(pair, ":")
		if !ok {
			return V5TaskRecord{}, fmt.Errorf("invalid v5 metadata pair %q", pair)
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if !v5MetadataKeys[key] {
			return V5TaskRecord{}, fmt.Errorf("unknown v5 metadata field %q", key)
		}
		if _, duplicate := values[key]; duplicate {
			return V5TaskRecord{}, fmt.Errorf("duplicate v5 metadata field %q", key)
		}
		values[key] = value
	}
	for _, required := range []string{"id", "short_id", "state", "priority", "created", "updated"} {
		if values[required] == "" {
			return V5TaskRecord{}, fmt.Errorf("missing required v5 metadata field %q", required)
		}
	}

	shortID, err := strconv.Atoi(values["short_id"])
	if err != nil {
		return V5TaskRecord{}, fmt.Errorf("parse v5 short_id: %w", err)
	}
	priority, err := strconv.Atoi(values["priority"])
	if err != nil {
		return V5TaskRecord{}, fmt.Errorf("parse v5 priority: %w", err)
	}
	created, err := parseV5Time(values["created"])
	if err != nil {
		return V5TaskRecord{}, fmt.Errorf("parse v5 created: %w", err)
	}
	updated, err := parseV5Time(values["updated"])
	if err != nil {
		return V5TaskRecord{}, fmt.Errorf("parse v5 updated: %w", err)
	}

	task := V5TaskRecord{
		ID:           values["id"],
		ShortID:      shortID,
		State:        values["state"],
		Priority:     priority,
		ParentID:     values["parent"],
		Tags:         splitV5Set(values["tags"]),
		CreatedAt:    created,
		UpdatedAt:    updated,
		DueDate:      values["due"],
		SnoozedUntil: values["snoozed"],
		DependsOn:    splitV5Set(values["depends"]),
	}
	task.Tags, err = normalizeMigrationV5Tags(task.Tags)
	if err != nil {
		return V5TaskRecord{}, fmt.Errorf("parse v5 tags: %w", err)
	}
	if values["completed"] != "" {
		parsed, parseErr := parseV5Time(values["completed"])
		if parseErr != nil {
			return V5TaskRecord{}, fmt.Errorf("parse v5 completed: %w", parseErr)
		}
		task.CompletedAt = &parsed
	}
	if values["pinned"] != "" {
		if values["pinned"] != "true" {
			return V5TaskRecord{}, fmt.Errorf("v5 pinned must be true when present")
		}
		task.Pinned = true
	}
	if values["pinned_at"] != "" {
		parsed, parseErr := parseV5Time(values["pinned_at"])
		if parseErr != nil {
			return V5TaskRecord{}, fmt.Errorf("parse v5 pinned_at: %w", parseErr)
		}
		task.PinnedAt = &parsed
	}
	if values["recur"] != "" {
		recurrence, parseErr := parseV5Recurrence(values["recur"])
		if parseErr != nil {
			return V5TaskRecord{}, parseErr
		}
		task.Recurrence = &recurrence
	}
	if values["attachments"] != "" {
		task.Attachments, err = decodeV5Attachments(values["attachments"])
		if err != nil {
			return V5TaskRecord{}, err
		}
	}
	if values["checklist"] != "" {
		task.Checklist, err = decodeV5Checklist(values["checklist"])
		if err != nil {
			return V5TaskRecord{}, err
		}
	}
	if err := validateV5TaskRecord(task, false); err != nil {
		return V5TaskRecord{}, err
	}
	return task, nil
}

func formatV5Time(value time.Time) string {
	return value.Format(time.RFC3339Nano)
}

func parseV5Time(value string) (time.Time, error) {
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return time.Time{}, err
	}
	if formatV5Time(parsed) != value {
		return time.Time{}, fmt.Errorf("timestamp %q is not canonical RFC3339Nano", value)
	}
	return parsed, nil
}

func splitV5Set(value string) []string {
	if value == "" {
		return nil
	}
	return strings.Split(value, ";")
}
