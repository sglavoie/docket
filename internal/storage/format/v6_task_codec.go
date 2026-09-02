package format

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// This file owns the schema-6 record shape and its metadata comment: the one
// line that carries a task's scalar fields. Record validation lives in
// v6_task_validate.go, the compound field encodings in v6_task_fields.go, and
// the surrounding rich-text blocks in v6_task_body.go. Later schemas that
// changed only how a field is spelled reuse the parameterized helpers here
// rather than forking them.

const v6DateFormat = "2006-01-02"

// metadataKeyDeferredLegacy is the durable metadata key schema 8 and every
// earlier schema use for the deferral date. It is a parameter of the shared
// codec rather than a literal so schema 9 can spell the same fact differently
// without forking the one place durable key order lives.
const metadataKeyDeferredLegacy = "snoozed"

// metadataKeyDeferred is the schema-9 spelling of the deferral-date metadata
// key. It is the whole schema-9 change: the record shape, the field set, and
// every invariant are schema 8's.
const metadataKeyDeferred = "deferred"

// deferEncoding is how one schema spells its deferral value on disk: the
// metadata key it writes and the layout its value is canonical in. The shared
// codec takes it as a value rather than a key so a schema changing either half
// stays one declaration here.
type deferEncoding struct{ key, layout string }

var (
	deferDateLegacy  = deferEncoding{key: metadataKeyDeferredLegacy, layout: v6DateFormat}
	deferDateRenamed = deferEncoding{key: metadataKeyDeferred, layout: v6DateFormat}
	deferInstant     = deferEncoding{key: metadataKeyDeferred, layout: time.RFC3339Nano}
)

var v6MetadataKeys = map[string]bool{
	"id": true, "short_id": true, "state": true, "priority": true,
	"parent": true, "tags": true, "created": true, "updated": true,
	"completed": true, "due": true, "snoozed": true,
	"pinned": true, "pinned_at": true, "attachments": true, "checklist": true,
}

// V6TaskRecord is the complete schema-6 storage shape. It is deliberately
// independent of both the live runtime model and the migration-only v5 record:
// retired relationship and legacy narrative-record data has no representation.
// Recurrence is absent from the live model entirely and remains schema-4/5
// migration input.
type V6TaskRecord struct {
	ID           string             `json:"id"`
	ShortID      int                `json:"short_id"`
	Title        string             `json:"title"`
	Description  string             `json:"description,omitempty"`
	State        string             `json:"state"`
	Priority     int                `json:"priority"`
	ParentID     string             `json:"parent_id,omitempty"`
	Tags         []string           `json:"tags,omitempty"`
	CreatedAt    time.Time          `json:"created_at"`
	UpdatedAt    time.Time          `json:"updated_at"`
	CompletedAt  *time.Time         `json:"completed_at,omitempty"`
	DueDate      string             `json:"due_date,omitempty"`
	SnoozedUntil string             `json:"snoozed_until,omitempty"`
	Pinned       bool               `json:"pinned,omitempty"`
	PinnedAt     *time.Time         `json:"pinned_at,omitempty"`
	Attachments  []V6TaskAttachment `json:"attachments,omitempty"`
	Checklist    []V6ChecklistItem  `json:"checklist,omitempty"`
	Comments     []V6TaskComment    `json:"comments,omitempty"`
}

type V6TaskAttachment struct {
	Kind         string     `json:"kind"`
	Filename     string     `json:"filename"`
	OriginalPath string     `json:"original_path,omitempty"`
	LineNumber   int        `json:"line_number,omitempty"`
	AddedAt      *time.Time `json:"added_at,omitempty"`
}

type V6ChecklistItem struct {
	Text string `json:"text"`
	Done bool   `json:"done"`
}

type V6TaskComment struct {
	Text      string    `json:"text"`
	CreatedAt time.Time `json:"created_at"`
}

// WriteV6TaskMetadata writes the canonical schema-6 metadata comment.
func WriteV6TaskMetadata(task V6TaskRecord) (string, error) {
	if err := validateV6TaskRecord(task, false, deferDateLegacy); err != nil {
		return "", err
	}
	return wrapTaskMetadata(v6MetadataParts(task)), nil
}

func wrapTaskMetadata(parts []string) string {
	return "<!-- " + strings.Join(parts, ", ") + " -->"
}

func v6MetadataParts(task V6TaskRecord) []string {
	return taskMetadataParts(task, deferDateLegacy, nil)
}

// taskMetadataParts renders the shared metadata fields. deferral names the
// deferral-date key in the caller's own schema. afterUpdated carries fields a
// later schema inserts next to the other timestamps rather than at the end of
// the comment.
func taskMetadataParts(task V6TaskRecord, deferral deferEncoding, afterUpdated []string) []string {
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
		"created: "+formatV6Time(task.CreatedAt),
		"updated: "+formatV6Time(task.UpdatedAt),
	)
	parts = append(parts, afterUpdated...)
	if task.CompletedAt != nil {
		parts = append(parts, "completed: "+formatV6Time(*task.CompletedAt))
	}
	if task.DueDate != "" {
		parts = append(parts, "due: "+task.DueDate)
	}
	if task.SnoozedUntil != "" {
		parts = append(parts, deferral.key+": "+task.SnoozedUntil)
	}
	if task.Pinned {
		parts = append(parts, "pinned: true")
	}
	if task.PinnedAt != nil {
		parts = append(parts, "pinned_at: "+formatV6Time(*task.PinnedAt))
	}
	if len(task.Attachments) > 0 {
		parts = append(parts, "attachments: "+encodeV6Attachments(task.Attachments))
	}
	if len(task.Checklist) > 0 {
		parts = append(parts, "checklist: "+encodeV6Checklist(task.Checklist))
	}
	return parts
}

// ParseV6TaskMetadata strictly parses one schema-6 metadata comment. Unknown
// fields, including every field retired from schema 5, are rejected.
func ParseV6TaskMetadata(comment string) (V6TaskRecord, error) {
	values, err := parseTaskMetadataFields("v6", comment, v6MetadataKeys, v6RequiredMetadataKeys)
	if err != nil {
		return V6TaskRecord{}, err
	}
	task, err := v6RecordFromMetadataValues(values, deferDateLegacy)
	if err != nil {
		return V6TaskRecord{}, err
	}
	if err := validateV6TaskRecord(task, false, deferDateLegacy); err != nil {
		return V6TaskRecord{}, err
	}
	return task, nil
}

var v6RequiredMetadataKeys = []string{"id", "short_id", "state", "priority", "created", "updated"}

// parseTaskMetadataFields strictly splits one metadata comment into its
// key/value pairs. schema names the schema in every error so a later schema
// reusing this kernel reports its own version.
func parseTaskMetadataFields(schema, comment string, allowed map[string]bool, required []string) (map[string]string, error) {
	match := MetadataRegex().FindStringSubmatch(strings.TrimSpace(comment))
	if match == nil {
		return nil, fmt.Errorf("invalid %s task metadata comment", schema)
	}
	values := make(map[string]string)
	for _, pair := range strings.Split(match[1], ", ") {
		key, value, ok := strings.Cut(pair, ":")
		if !ok {
			return nil, fmt.Errorf("invalid %s metadata pair %q", schema, pair)
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if !allowed[key] {
			return nil, fmt.Errorf("unknown %s metadata field %q", schema, key)
		}
		if _, duplicate := values[key]; duplicate {
			return nil, fmt.Errorf("duplicate %s metadata field %q", schema, key)
		}
		values[key] = value
	}
	for _, key := range required {
		if values[key] == "" {
			return nil, fmt.Errorf("missing required %s metadata field %q", schema, key)
		}
	}
	return values, nil
}

// v6RecordFromMetadataValues decodes the shared metadata fields. deferral names
// the deferral-date key in the caller's own schema.
func v6RecordFromMetadataValues(values map[string]string, deferral deferEncoding) (V6TaskRecord, error) {
	shortID, err := strconv.Atoi(values["short_id"])
	if err != nil {
		return V6TaskRecord{}, fmt.Errorf("parse v6 short_id: %w", err)
	}
	priority, err := strconv.Atoi(values["priority"])
	if err != nil {
		return V6TaskRecord{}, fmt.Errorf("parse v6 priority: %w", err)
	}
	created, err := parseV6Time(values["created"])
	if err != nil {
		return V6TaskRecord{}, fmt.Errorf("parse v6 created: %w", err)
	}
	updated, err := parseV6Time(values["updated"])
	if err != nil {
		return V6TaskRecord{}, fmt.Errorf("parse v6 updated: %w", err)
	}

	task := V6TaskRecord{
		ID:           values["id"],
		ShortID:      shortID,
		State:        values["state"],
		Priority:     priority,
		ParentID:     values["parent"],
		Tags:         splitV6Set(values["tags"]),
		CreatedAt:    created,
		UpdatedAt:    updated,
		DueDate:      values["due"],
		SnoozedUntil: values[deferral.key],
	}
	if values["completed"] != "" {
		parsed, parseErr := parseV6Time(values["completed"])
		if parseErr != nil {
			return V6TaskRecord{}, fmt.Errorf("parse v6 completed: %w", parseErr)
		}
		task.CompletedAt = &parsed
	}
	if values["pinned"] != "" {
		if values["pinned"] != "true" {
			return V6TaskRecord{}, fmt.Errorf("v6 pinned must be true when present")
		}
		task.Pinned = true
	}
	if values["pinned_at"] != "" {
		parsed, parseErr := parseV6Time(values["pinned_at"])
		if parseErr != nil {
			return V6TaskRecord{}, fmt.Errorf("parse v6 pinned_at: %w", parseErr)
		}
		task.PinnedAt = &parsed
	}
	if values["attachments"] != "" {
		task.Attachments, err = decodeV6Attachments(values["attachments"])
		if err != nil {
			return V6TaskRecord{}, err
		}
	}
	if values["checklist"] != "" {
		task.Checklist, err = decodeV6Checklist(values["checklist"])
		if err != nil {
			return V6TaskRecord{}, err
		}
	}
	return task, nil
}

func formatV6Time(value time.Time) string {
	return value.Format(time.RFC3339Nano)
}

func parseV6Time(value string) (time.Time, error) {
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return time.Time{}, err
	}
	if formatV6Time(parsed) != value {
		return time.Time{}, fmt.Errorf("timestamp %q is not canonical RFC3339Nano", value)
	}
	return parsed, nil
}

func splitV6Set(value string) []string {
	if value == "" {
		return nil
	}
	return strings.Split(value, ";")
}
