package format

import (
	"fmt"
	"strconv"
	"strings"
)

// V13TaskRecord is the complete schema-13 storage shape: the schema-12 fields
// plus an optional due-date clock time. The due date itself stays in the
// strict, date-only "due" field (validated as YYYY-MM-DD by every schema down
// to v6); the new "due_time" field carries the hh:mm a user attached to it, so
// a date-only due date is exactly a record with no due_time. Splitting the time
// into its own additive key keeps the older date-only field and its validators
// unchanged, mirroring how schema 12 added defer_count.
type V13TaskRecord struct {
	V12TaskRecord
	DueTime string `json:"due_time,omitempty"`
}

var v13MetadataKeys = v13TaskMetadataKeys()

func v13TaskMetadataKeys() map[string]bool {
	keys := make(map[string]bool, len(v12MetadataKeys)+1)
	for key := range v12MetadataKeys {
		keys[key] = true
	}
	keys["due_time"] = true
	return keys
}

// DueTime is optional, so schema 13 has the same required metadata as schema 12.
var v13RequiredMetadataKeys = v12RequiredMetadataKeys

// ValidateV13TaskRecord validates a complete schema-13 task record.
func ValidateV13TaskRecord(task V13TaskRecord) error {
	return validateV13TaskRecord(task, true)
}

func validateV13TaskRecord(task V13TaskRecord, requireTitle bool) error {
	if err := validateV12TaskRecord(task.V12TaskRecord, requireTitle); err != nil {
		return err
	}
	if task.DueTime == "" {
		return nil
	}
	// A clock time is meaningless without a day, so it may appear only beside a
	// due date. The date remains the date-only field the older validators own.
	if task.DueDate == "" {
		return fmt.Errorf("v13 task %q has a due_time %q without a due date", task.ID, task.DueTime)
	}
	if err := validateV13DueTime(task.DueTime); err != nil {
		return fmt.Errorf("v13 task %q: %w", task.ID, err)
	}
	return nil
}

// validateV13DueTime enforces a canonical zero-padded HH:MM in 00:00-23:59, so
// the field cannot introduce a metadata delimiter or a value the live model
// cannot represent.
func validateV13DueTime(value string) error {
	if len(value) != 5 || value[2] != ':' {
		return fmt.Errorf("due_time %q is not canonical HH:MM", value)
	}
	hour, err := strconv.Atoi(value[:2])
	if err != nil || hour < 0 || hour > 23 {
		return fmt.Errorf("due_time %q is not canonical HH:MM", value)
	}
	minute, err := strconv.Atoi(value[3:])
	if err != nil || minute < 0 || minute > 59 {
		return fmt.Errorf("due_time %q is not canonical HH:MM", value)
	}
	// Midnight is the date-only signal, so it is never persisted as a due_time.
	if hour == 0 && minute == 0 {
		return fmt.Errorf("due_time %q is midnight; a date-only due date omits due_time", value)
	}
	return nil
}

// WriteV13TaskMetadata writes the canonical schema-13 metadata comment.
func WriteV13TaskMetadata(task V13TaskRecord) (string, error) {
	if err := validateV13TaskRecord(task, false); err != nil {
		return "", err
	}
	extra := []string{"state_changed: " + formatV6Time(task.StateChangedAt)}
	parts := taskMetadataParts(task.V6TaskRecord, deferInstant, extra)
	if task.Context != "" {
		parts = append(parts, "context: "+task.Context)
	}
	if task.DeferCount > 0 {
		parts = append(parts, "defer_count: "+strconv.Itoa(task.DeferCount))
	}
	if task.DueTime != "" {
		parts = append(parts, "due_time: "+task.DueTime)
	}
	return wrapTaskMetadata(parts), nil
}

// ParseV13TaskMetadata strictly parses one schema-13 metadata comment.
func ParseV13TaskMetadata(comment string) (V13TaskRecord, error) {
	values, err := parseTaskMetadataFields("v13", comment, v13MetadataKeys, v13RequiredMetadataKeys)
	if err != nil {
		return V13TaskRecord{}, err
	}
	base, err := v6RecordFromMetadataValues(values, deferInstant)
	if err != nil {
		return V13TaskRecord{}, err
	}
	stateChangedAt, err := parseV6Time(values["state_changed"])
	if err != nil {
		return V13TaskRecord{}, fmt.Errorf("parse v13 state_changed: %w", err)
	}
	deferCount := 0
	if value, present := values["defer_count"]; present {
		deferCount, err = strconv.Atoi(value)
		if err != nil {
			return V13TaskRecord{}, fmt.Errorf("parse v13 defer_count: %w", err)
		}
		if deferCount == 0 {
			return V13TaskRecord{}, fmt.Errorf("parse v13 defer_count: value must be positive when present")
		}
	}
	task := V13TaskRecord{
		V12TaskRecord: V12TaskRecord{
			V11TaskRecord: V11TaskRecord{
				V10TaskRecord: V10TaskRecord{V6TaskRecord: base, StateChangedAt: stateChangedAt},
				Context:       values["context"],
			},
			DeferCount: deferCount,
		},
		DueTime: values["due_time"],
	}
	if err := validateV13TaskRecord(task, false); err != nil {
		return V13TaskRecord{}, err
	}
	return task, nil
}

// WriteV13TaskBody renders metadata plus the retained description and comment
// blocks. The project document owns the generic Task heading.
func WriteV13TaskBody(task V13TaskRecord) (string, error) {
	if err := ValidateV13TaskRecord(task); err != nil {
		return "", err
	}
	metadata, err := WriteV13TaskMetadata(task)
	if err != nil {
		return "", err
	}
	return writeTaskBody(metadata, task.Description, task.Comments), nil
}

// ParseV13TaskBody parses the body below one generic Task heading.
func ParseV13TaskBody(title, body string) (V13TaskRecord, error) {
	lines := strings.Split(strings.TrimSuffix(body, "\n"), "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) == "" {
		return V13TaskRecord{}, fmt.Errorf("v13 task %q is missing metadata", title)
	}
	task, err := ParseV13TaskMetadata(lines[0])
	if err != nil {
		return V13TaskRecord{}, fmt.Errorf("v13 task %q: %w", title, err)
	}
	task.Title = title

	task.Description, task.Comments, err = parseTaskBodyBlocks("v13", title, lines)
	if err != nil {
		return V13TaskRecord{}, err
	}
	if err := ValidateV13TaskRecord(task); err != nil {
		return V13TaskRecord{}, err
	}
	return task, nil
}
