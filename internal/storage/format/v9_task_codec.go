package format

import (
	"fmt"
	"strings"
)

// metadataKeyDeferred is the schema-9 spelling of the deferral-date metadata
// key. It is the whole schema-9 change: the record shape, the field set, and
// every invariant are schema 8's.
const metadataKeyDeferred = "deferred"

// V9TaskRecord is the complete schema-9 storage shape. Schema 9 renames a
// durable metadata key and adds no field, so it is an alias rather than a new
// struct: a DeferredUntil field beside the inherited SnoozedUntil would let two
// names claim one fact.
type V9TaskRecord = V8TaskRecord

var v9MetadataKeys = v9TaskMetadataKeys()

func v9TaskMetadataKeys() map[string]bool {
	keys := make(map[string]bool, len(v8MetadataKeys))
	for key := range v8MetadataKeys {
		keys[key] = true
	}
	// Deleting the legacy key is what makes the schema-9 parser reject a
	// schema-8 comment outright. The key is optional, so a schema-9 parser that
	// still allowed it would parse pre-rename documents and silently drop every
	// deferral date instead.
	delete(keys, metadataKeyDeferredLegacy)
	keys[metadataKeyDeferred] = true
	return keys
}

// v9RequiredMetadataKeys is the schema-8 list unchanged: the deferral date is
// absent from a task that was never deferred.
var v9RequiredMetadataKeys = v8RequiredMetadataKeys

// ValidateV9TaskRecord validates a complete schema-9 task record. Schema 9
// changes a durable key and no invariant, so the schema-8 rules apply verbatim
// apart from the key a rejected deferral date is named by.
func ValidateV9TaskRecord(task V9TaskRecord) error {
	return validateV8TaskRecord(task, true, metadataKeyDeferred)
}

// WriteV9TaskMetadata writes the canonical schema-9 metadata comment.
func WriteV9TaskMetadata(task V9TaskRecord) (string, error) {
	if err := validateV8TaskRecord(task, false, metadataKeyDeferred); err != nil {
		return "", err
	}
	extra := []string{"state_changed: " + formatV6Time(task.StateChangedAt)}
	return wrapTaskMetadata(taskMetadataParts(task.V6TaskRecord, metadataKeyDeferred, extra)), nil
}

// ParseV9TaskMetadata strictly parses one schema-9 metadata comment.
func ParseV9TaskMetadata(comment string) (V9TaskRecord, error) {
	values, err := parseTaskMetadataFields("v9", comment, v9MetadataKeys, v9RequiredMetadataKeys)
	if err != nil {
		return V9TaskRecord{}, err
	}
	base, err := v6RecordFromMetadataValues(values, metadataKeyDeferred)
	if err != nil {
		return V9TaskRecord{}, err
	}
	stateChangedAt, err := parseV6Time(values["state_changed"])
	if err != nil {
		return V9TaskRecord{}, fmt.Errorf("parse v9 state_changed: %w", err)
	}
	task := V9TaskRecord{V6TaskRecord: base, StateChangedAt: stateChangedAt}
	if err := validateV8TaskRecord(task, false, metadataKeyDeferred); err != nil {
		return V9TaskRecord{}, err
	}
	return task, nil
}

// WriteV9TaskBody renders metadata plus the retained description and comment
// blocks. The project document owns the generic Task heading.
func WriteV9TaskBody(task V9TaskRecord) (string, error) {
	if err := ValidateV9TaskRecord(task); err != nil {
		return "", err
	}
	metadata, err := WriteV9TaskMetadata(task)
	if err != nil {
		return "", err
	}
	return writeTaskBody(metadata, task.Description, task.Comments), nil
}

// ParseV9TaskBody parses the body below one generic Task heading.
func ParseV9TaskBody(title, body string) (V9TaskRecord, error) {
	lines := strings.Split(strings.TrimSuffix(body, "\n"), "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) == "" {
		return V9TaskRecord{}, fmt.Errorf("v9 task %q is missing metadata", title)
	}
	task, err := ParseV9TaskMetadata(lines[0])
	if err != nil {
		return V9TaskRecord{}, fmt.Errorf("v9 task %q: %w", title, err)
	}
	task.Title = title

	task.Description, task.Comments, err = parseTaskBodyBlocks("v9", title, lines)
	if err != nil {
		return V9TaskRecord{}, err
	}
	if err := ValidateV9TaskRecord(task); err != nil {
		return V9TaskRecord{}, err
	}
	return task, nil
}
