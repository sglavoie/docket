package format

import (
	"fmt"
	"strings"
)

// V19TaskRecord is the complete schema-19 storage shape: the schema-18 fields
// plus the stored hidden flag. Hidden is an axis beside the pin and the review
// flag rather than a state or a collection: it moves a task out of every view
// without changing what the task is, so it needs a field of its own.
//
// The flag is optional and written only when true, so a task that is not hidden
// stays byte-identical to its schema-18 predecessor. That is the same additive
// discipline review, the resume note, and the check-in were added under.
type V19TaskRecord struct {
	V18TaskRecord
	Hidden bool
}

// v19HiddenMetadataKey is the one key schema 19 adds.
const v19HiddenMetadataKey = "hidden"

// v19HiddenTrue is the only value the hidden key is ever written with. Parsing
// is strict about it, as it is for review: the writer omits the key entirely
// when the flag is off, so any other spelling is a malformed record.
const v19HiddenTrue = "true"

var v19MetadataKeys = v19TaskMetadataKeys()

func v19TaskMetadataKeys() map[string]bool {
	keys := make(map[string]bool, len(v18MetadataKeys)+1)
	for key := range v18MetadataKeys {
		keys[key] = true
	}
	keys[v19HiddenMetadataKey] = true
	return keys
}

// The hidden flag is optional, so schema 19 has the same required metadata as
// schema 18.
var v19RequiredMetadataKeys = v18RequiredMetadataKeys

// ValidateV19TaskRecord validates a complete schema-19 task record. The flag is
// a boolean, so it adds nothing to validate beyond the inherited fields.
func ValidateV19TaskRecord(task V19TaskRecord) error {
	return ValidateV18TaskRecord(task.V18TaskRecord)
}

// WriteV19TaskMetadata writes the canonical schema-19 metadata comment. A false
// hidden flag omits the key entirely, which is what keeps a migrated record
// byte-equal to the schema-18 record it came from.
func WriteV19TaskMetadata(task V19TaskRecord) (string, error) {
	if err := validateV18TaskRecord(task.V18TaskRecord, false); err != nil {
		return "", err
	}
	parts := v18MetadataParts(task.V18TaskRecord)
	if task.Hidden {
		parts = append(parts, v19HiddenMetadataKey+": "+v19HiddenTrue)
	}
	return wrapTaskMetadata(parts), nil
}

// ParseV19TaskMetadata strictly parses one schema-19 metadata comment.
func ParseV19TaskMetadata(comment string) (V19TaskRecord, error) {
	values, err := parseTaskMetadataFields("v19", comment, v19MetadataKeys, v19RequiredMetadataKeys)
	if err != nil {
		return V19TaskRecord{}, err
	}
	base, err := v18RecordFromMetadataValues(values)
	if err != nil {
		return V19TaskRecord{}, err
	}
	task := V19TaskRecord{V18TaskRecord: base}
	if raw, present := values[v19HiddenMetadataKey]; present {
		if raw != v19HiddenTrue {
			return V19TaskRecord{}, fmt.Errorf("parse v19 hidden: value must be %q when present, got %q", v19HiddenTrue, raw)
		}
		task.Hidden = true
	}
	if err := validateV18TaskRecord(task.V18TaskRecord, false); err != nil {
		return V19TaskRecord{}, err
	}
	return task, nil
}

// WriteV19TaskBody renders metadata plus the optional checklist-origin line and
// the retained resume note, description, and comment blocks. Only the metadata
// line differs from schema 18, so the blocks and the origin line delegate to
// the same shared helpers WriteV18TaskBody uses.
func WriteV19TaskBody(task V19TaskRecord) (string, error) {
	if err := ValidateV19TaskRecord(task); err != nil {
		return "", err
	}
	metadata, err := WriteV19TaskMetadata(task)
	if err != nil {
		return "", err
	}
	body := writeTaskBodyV16(metadata, task.ResumeNote, task.Description, task.Comments)
	return insertChecklistOriginLine(body, task.ChecklistOrigin)
}

// ParseV19TaskBody parses the body below one generic Task heading.
func ParseV19TaskBody(title, body string) (V19TaskRecord, error) {
	origin, remainder, err := splitChecklistOriginLine("v19", title, body)
	if err != nil {
		return V19TaskRecord{}, err
	}
	lines := strings.Split(strings.TrimSuffix(remainder, "\n"), "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) == "" {
		return V19TaskRecord{}, fmt.Errorf("v19 task %q is missing metadata", title)
	}
	task, err := ParseV19TaskMetadata(lines[0])
	if err != nil {
		return V19TaskRecord{}, fmt.Errorf("v19 task %q: %w", title, err)
	}
	task.Title = title
	task.ChecklistOrigin = origin

	task.ResumeNote, task.Description, task.Comments, err = parseTaskBodyBlocksV16("v19", title, lines)
	if err != nil {
		return V19TaskRecord{}, err
	}
	if err := ValidateV19TaskRecord(task); err != nil {
		return V19TaskRecord{}, err
	}
	return task, nil
}
