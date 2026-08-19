package format

import (
	"fmt"
	"strconv"
	"strings"
)

// V12TaskRecord is the complete schema-12 storage shape: the schema-11 fields
// plus the number of times the task has been deferred.
type V12TaskRecord struct {
	V11TaskRecord
	DeferCount int `json:"defer_count,omitempty"`
}

var v12MetadataKeys = v12TaskMetadataKeys()

func v12TaskMetadataKeys() map[string]bool {
	keys := make(map[string]bool, len(v11MetadataKeys)+1)
	for key := range v11MetadataKeys {
		keys[key] = true
	}
	keys["defer_count"] = true
	return keys
}

// DeferCount is optional, so schema 12 has the same required metadata as schema 11.
var v12RequiredMetadataKeys = v11RequiredMetadataKeys

// ValidateV12TaskRecord validates a complete schema-12 task record.
func ValidateV12TaskRecord(task V12TaskRecord) error {
	return validateV12TaskRecord(task, true)
}

func validateV12TaskRecord(task V12TaskRecord, requireTitle bool) error {
	if err := validateV11TaskRecord(task.V11TaskRecord, requireTitle); err != nil {
		return err
	}
	// The writer emits DeferCount with strconv.Itoa and the parser accepts only a
	// decimal int. Together with this range check, the field cannot introduce a
	// metadata delimiter or a value outside the live model's representation.
	if task.DeferCount < 0 {
		return fmt.Errorf("v12 task %q has negative defer count %d", task.ID, task.DeferCount)
	}
	return nil
}

// WriteV12TaskMetadata writes the canonical schema-12 metadata comment.
func WriteV12TaskMetadata(task V12TaskRecord) (string, error) {
	if err := validateV12TaskRecord(task, false); err != nil {
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
	return wrapTaskMetadata(parts), nil
}

// ParseV12TaskMetadata strictly parses one schema-12 metadata comment.
func ParseV12TaskMetadata(comment string) (V12TaskRecord, error) {
	values, err := parseTaskMetadataFields("v12", comment, v12MetadataKeys, v12RequiredMetadataKeys)
	if err != nil {
		return V12TaskRecord{}, err
	}
	base, err := v6RecordFromMetadataValues(values, deferInstant)
	if err != nil {
		return V12TaskRecord{}, err
	}
	stateChangedAt, err := parseV6Time(values["state_changed"])
	if err != nil {
		return V12TaskRecord{}, fmt.Errorf("parse v12 state_changed: %w", err)
	}
	deferCount := 0
	if value, present := values["defer_count"]; present {
		deferCount, err = strconv.Atoi(value)
		if err != nil {
			return V12TaskRecord{}, fmt.Errorf("parse v12 defer_count: %w", err)
		}
		if deferCount == 0 {
			return V12TaskRecord{}, fmt.Errorf("parse v12 defer_count: value must be positive when present")
		}
	}
	task := V12TaskRecord{
		V11TaskRecord: V11TaskRecord{
			V10TaskRecord: V10TaskRecord{V6TaskRecord: base, StateChangedAt: stateChangedAt},
			Context:       values["context"],
		},
		DeferCount: deferCount,
	}
	if err := validateV12TaskRecord(task, false); err != nil {
		return V12TaskRecord{}, err
	}
	return task, nil
}

// WriteV12TaskBody renders metadata plus the retained description and comment
// blocks. The project document owns the generic Task heading.
func WriteV12TaskBody(task V12TaskRecord) (string, error) {
	if err := ValidateV12TaskRecord(task); err != nil {
		return "", err
	}
	metadata, err := WriteV12TaskMetadata(task)
	if err != nil {
		return "", err
	}
	return writeTaskBody(metadata, task.Description, task.Comments), nil
}

// ParseV12TaskBody parses the body below one generic Task heading.
func ParseV12TaskBody(title, body string) (V12TaskRecord, error) {
	lines := strings.Split(strings.TrimSuffix(body, "\n"), "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) == "" {
		return V12TaskRecord{}, fmt.Errorf("v12 task %q is missing metadata", title)
	}
	task, err := ParseV12TaskMetadata(lines[0])
	if err != nil {
		return V12TaskRecord{}, fmt.Errorf("v12 task %q: %w", title, err)
	}
	task.Title = title

	task.Description, task.Comments, err = parseTaskBodyBlocks("v12", title, lines)
	if err != nil {
		return V12TaskRecord{}, err
	}
	if err := ValidateV12TaskRecord(task); err != nil {
		return V12TaskRecord{}, err
	}
	return task, nil
}
