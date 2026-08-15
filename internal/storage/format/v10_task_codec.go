package format

import (
	"fmt"
	"strings"
)

// V10TaskRecord is the complete schema-10 storage shape. Schema 10 changes the
// layout of one durable value and adds no field, so it is an alias rather than
// a new struct, following the schema-9 precedent.
type V10TaskRecord = V9TaskRecord

// v10MetadataKeys is the schema-9 key set unchanged: schema 10 keeps writing
// the deferral value under "deferred" and changes only the layout behind it, so
// a schema-9 comment is rejected by the value check rather than the key check.
var v10MetadataKeys = v9MetadataKeys

// v10RequiredMetadataKeys is the schema-9 list unchanged: the deferral instant
// is absent from a task that was never deferred.
var v10RequiredMetadataKeys = v9RequiredMetadataKeys

// ValidateV10TaskRecord validates a complete schema-10 task record. Schema 10
// changes a durable value layout and no invariant, so the schema-8 rules apply
// verbatim apart from the layout a deferral value must be canonical in.
func ValidateV10TaskRecord(task V10TaskRecord) error {
	return validateV8TaskRecord(task, true, deferInstant)
}

// WriteV10TaskMetadata writes the canonical schema-10 metadata comment.
func WriteV10TaskMetadata(task V10TaskRecord) (string, error) {
	if err := validateV8TaskRecord(task, false, deferInstant); err != nil {
		return "", err
	}
	extra := []string{"state_changed: " + formatV6Time(task.StateChangedAt)}
	return wrapTaskMetadata(taskMetadataParts(task.V6TaskRecord, deferInstant, extra)), nil
}

// ParseV10TaskMetadata strictly parses one schema-10 metadata comment.
func ParseV10TaskMetadata(comment string) (V10TaskRecord, error) {
	values, err := parseTaskMetadataFields("v10", comment, v10MetadataKeys, v10RequiredMetadataKeys)
	if err != nil {
		return V10TaskRecord{}, err
	}
	base, err := v6RecordFromMetadataValues(values, deferInstant)
	if err != nil {
		return V10TaskRecord{}, err
	}
	stateChangedAt, err := parseV6Time(values["state_changed"])
	if err != nil {
		return V10TaskRecord{}, fmt.Errorf("parse v10 state_changed: %w", err)
	}
	task := V10TaskRecord{V6TaskRecord: base, StateChangedAt: stateChangedAt}
	if err := validateV8TaskRecord(task, false, deferInstant); err != nil {
		return V10TaskRecord{}, err
	}
	return task, nil
}

// WriteV10TaskBody renders metadata plus the retained description and comment
// blocks. The project document owns the generic Task heading.
func WriteV10TaskBody(task V10TaskRecord) (string, error) {
	if err := ValidateV10TaskRecord(task); err != nil {
		return "", err
	}
	metadata, err := WriteV10TaskMetadata(task)
	if err != nil {
		return "", err
	}
	return writeTaskBody(metadata, task.Description, task.Comments), nil
}

// ParseV10TaskBody parses the body below one generic Task heading.
func ParseV10TaskBody(title, body string) (V10TaskRecord, error) {
	lines := strings.Split(strings.TrimSuffix(body, "\n"), "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) == "" {
		return V10TaskRecord{}, fmt.Errorf("v10 task %q is missing metadata", title)
	}
	task, err := ParseV10TaskMetadata(lines[0])
	if err != nil {
		return V10TaskRecord{}, fmt.Errorf("v10 task %q: %w", title, err)
	}
	task.Title = title

	task.Description, task.Comments, err = parseTaskBodyBlocks("v10", title, lines)
	if err != nil {
		return V10TaskRecord{}, err
	}
	if err := ValidateV10TaskRecord(task); err != nil {
		return V10TaskRecord{}, err
	}
	return task, nil
}
