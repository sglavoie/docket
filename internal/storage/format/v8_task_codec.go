package format

import (
	"fmt"
	"strings"
	"time"
)

// V8TaskRecord is the complete schema-8 storage shape: the schema-7 fields plus
// the state clock. Embedding keeps the schema lineage explicit, so a field
// retired from schema 7 cannot silently reappear here.
type V8TaskRecord struct {
	V6TaskRecord
	// StateChangedAt is the instant the task's state last changed. It is
	// distinct from UpdatedAt, which any edit bumps.
	StateChangedAt time.Time `json:"state_changed_at"`
}

var v8MetadataKeys = v8TaskMetadataKeys()

func v8TaskMetadataKeys() map[string]bool {
	keys := make(map[string]bool, len(v6MetadataKeys)+1)
	for key := range v6MetadataKeys {
		keys[key] = true
	}
	keys["state_changed"] = true
	return keys
}

var v8RequiredMetadataKeys = append(append([]string{}, v6RequiredMetadataKeys...), "state_changed")

// V8TaskRecordFromV7 lifts a schema-7 record to schema 8 by backfilling the
// state clock from UpdatedAt. That is an approximation: UpdatedAt is the last
// touch of any field, so the clock reads correctly only until the task's next
// real state transition, which overwrites it with the true instant.
func V8TaskRecordFromV7(task V6TaskRecord) V8TaskRecord {
	return V8TaskRecord{V6TaskRecord: task, StateChangedAt: task.UpdatedAt}
}

// ValidateV8TaskRecord validates a complete schema-8 task record.
func ValidateV8TaskRecord(task V8TaskRecord) error {
	return validateV8TaskRecord(task, true, deferDateLegacy)
}

func validateV8TaskRecord(task V8TaskRecord, requireTitle bool, deferral deferEncoding) error {
	if requireTitle {
		if err := validateV7TaskRecord(task.V6TaskRecord, deferral); err != nil {
			return err
		}
	} else if err := validateV6TaskRecord(task.V6TaskRecord, false, deferral); err != nil {
		return err
	}
	if task.StateChangedAt.IsZero() {
		return fmt.Errorf("v8 task %q requires a state changed timestamp", task.ID)
	}
	return nil
}

// WriteV8TaskMetadata writes the canonical schema-8 metadata comment.
func WriteV8TaskMetadata(task V8TaskRecord) (string, error) {
	if err := validateV8TaskRecord(task, false, deferDateLegacy); err != nil {
		return "", err
	}
	extra := []string{"state_changed: " + formatV6Time(task.StateChangedAt)}
	return wrapTaskMetadata(taskMetadataParts(task.V6TaskRecord, deferDateLegacy, extra)), nil
}

// ParseV8TaskMetadata strictly parses one schema-8 metadata comment.
func ParseV8TaskMetadata(comment string) (V8TaskRecord, error) {
	values, err := parseTaskMetadataFields("v8", comment, v8MetadataKeys, v8RequiredMetadataKeys)
	if err != nil {
		return V8TaskRecord{}, err
	}
	base, err := v6RecordFromMetadataValues(values, deferDateLegacy)
	if err != nil {
		return V8TaskRecord{}, err
	}
	stateChangedAt, err := parseV6Time(values["state_changed"])
	if err != nil {
		return V8TaskRecord{}, fmt.Errorf("parse v8 state_changed: %w", err)
	}
	task := V8TaskRecord{V6TaskRecord: base, StateChangedAt: stateChangedAt}
	if err := validateV8TaskRecord(task, false, deferDateLegacy); err != nil {
		return V8TaskRecord{}, err
	}
	return task, nil
}

// WriteV8TaskBody renders metadata plus the retained description and comment
// blocks. The project document owns the generic Task heading.
func WriteV8TaskBody(task V8TaskRecord) (string, error) {
	if err := ValidateV8TaskRecord(task); err != nil {
		return "", err
	}
	metadata, err := WriteV8TaskMetadata(task)
	if err != nil {
		return "", err
	}
	return writeTaskBody(metadata, task.Description, task.Comments), nil
}

// ParseV8TaskBody parses the body below one generic Task heading.
func ParseV8TaskBody(title, body string) (V8TaskRecord, error) {
	lines := strings.Split(strings.TrimSuffix(body, "\n"), "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) == "" {
		return V8TaskRecord{}, fmt.Errorf("v8 task %q is missing metadata", title)
	}
	task, err := ParseV8TaskMetadata(lines[0])
	if err != nil {
		return V8TaskRecord{}, fmt.Errorf("v8 task %q: %w", title, err)
	}
	task.Title = title

	task.Description, task.Comments, err = parseTaskBodyBlocks("v8", title, lines)
	if err != nil {
		return V8TaskRecord{}, err
	}
	if err := ValidateV8TaskRecord(task); err != nil {
		return V8TaskRecord{}, err
	}
	return task, nil
}
