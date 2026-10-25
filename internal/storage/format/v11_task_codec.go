package format

import (
	"fmt"
	"strings"

	"github.com/sglavoie/docket/internal/models"
)

// V11TaskRecord is the complete schema-11 storage shape: the schema-10 fields
// plus one optional, single-valued task context.
type V11TaskRecord struct {
	V10TaskRecord
	Context string `json:"context,omitempty"`
}

var v11MetadataKeys = v11TaskMetadataKeys()

func v11TaskMetadataKeys() map[string]bool {
	keys := make(map[string]bool, len(v10MetadataKeys)+1)
	for key := range v10MetadataKeys {
		keys[key] = true
	}
	keys["context"] = true
	return keys
}

// Context is optional, so schema 11 has the same required metadata as schema 10.
var v11RequiredMetadataKeys = v10RequiredMetadataKeys

// ValidateV11TaskRecord validates a complete schema-11 task record.
func ValidateV11TaskRecord(task V11TaskRecord) error {
	return validateV11TaskRecord(task, true)
}

func validateV11TaskRecord(task V11TaskRecord, requireTitle bool) error {
	if err := validateV8TaskRecord(task.V10TaskRecord, requireTitle, deferInstant); err != nil {
		return err
	}
	// The shared parser splits fields on ", " and cuts keys at the first ":".
	// Extend the older metadata delimiter guard with ':' so Context cannot
	// corrupt either boundary even if its canonical validator changes later.
	if strings.ContainsAny(task.Context, ",;:\n\r") {
		return fmt.Errorf("v11 task %q context %q contains a metadata delimiter", task.ID, task.Context)
	}
	if err := models.ValidateTaskContext(task.Context); err != nil {
		return fmt.Errorf("v11 task %q: %w", task.ID, err)
	}
	return nil
}

// WriteV11TaskMetadata writes the canonical schema-11 metadata comment.
func WriteV11TaskMetadata(task V11TaskRecord) (string, error) {
	if err := validateV11TaskRecord(task, false); err != nil {
		return "", err
	}
	extra := []string{"state_changed: " + formatV6Time(task.StateChangedAt)}
	parts := taskMetadataParts(task.V6TaskRecord, deferInstant, extra)
	if task.Context != "" {
		parts = append(parts, "context: "+task.Context)
	}
	return wrapTaskMetadata(parts), nil
}

// ParseV11TaskMetadata strictly parses one schema-11 metadata comment.
func ParseV11TaskMetadata(comment string) (V11TaskRecord, error) {
	values, err := parseTaskMetadataFields("v11", comment, v11MetadataKeys, v11RequiredMetadataKeys)
	if err != nil {
		return V11TaskRecord{}, err
	}
	base, err := v6RecordFromMetadataValues(values, deferInstant)
	if err != nil {
		return V11TaskRecord{}, err
	}
	stateChangedAt, err := parseV6Time(values["state_changed"])
	if err != nil {
		return V11TaskRecord{}, fmt.Errorf("parse v11 state_changed: %w", err)
	}
	task := V11TaskRecord{
		V10TaskRecord: V10TaskRecord{V6TaskRecord: base, StateChangedAt: stateChangedAt},
		Context:       values["context"],
	}
	if err := validateV11TaskRecord(task, false); err != nil {
		return V11TaskRecord{}, err
	}
	return task, nil
}

// WriteV11TaskBody renders metadata plus the retained description and comment
// blocks. The project document owns the generic Task heading.
func WriteV11TaskBody(task V11TaskRecord) (string, error) {
	if err := ValidateV11TaskRecord(task); err != nil {
		return "", err
	}
	metadata, err := WriteV11TaskMetadata(task)
	if err != nil {
		return "", err
	}
	return writeTaskBody(metadata, task.Description, task.Comments), nil
}

// ParseV11TaskBody parses the body below one generic Task heading.
func ParseV11TaskBody(title, body string) (V11TaskRecord, error) {
	lines := strings.Split(strings.TrimSuffix(body, "\n"), "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) == "" {
		return V11TaskRecord{}, fmt.Errorf("v11 task %q is missing metadata", title)
	}
	task, err := ParseV11TaskMetadata(lines[0])
	if err != nil {
		return V11TaskRecord{}, fmt.Errorf("v11 task %q: %w", title, err)
	}
	task.Title = title

	task.Description, task.Comments, err = parseTaskBodyBlocks("v11", title, lines)
	if err != nil {
		return V11TaskRecord{}, err
	}
	if err := ValidateV11TaskRecord(task); err != nil {
		return V11TaskRecord{}, err
	}
	return task, nil
}
