package format

import (
	"fmt"
	"strconv"
)

// V15TaskRecord is the complete schema-15 storage shape: the schema-14 fields
// plus the stored review flag. Review used to be spelled as an ordinary task tag
// named "review", which made a promoted lane depend on a value any user could
// mint or misspell. Schema 15 gives it a field of its own, beside the pin, so a
// lane reads a boolean rather than searching a tag list.
//
// The flag is optional and written only when true, so a record that is not up
// for review stays byte-identical to its schema-14 predecessor. That is the same
// additive discipline depends and due_time were added under.
type V15TaskRecord struct {
	V14TaskRecord
	Review bool
}

// v15ReviewMetadataKey is the one key schema 15 adds.
const v15ReviewMetadataKey = "review"

// v15ReviewTrue is the only value the review key is ever written with. Parsing
// is strict about it: a key present with any other spelling is a malformed
// record, not a false reading, because the writer omits the key entirely when
// the flag is off.
const v15ReviewTrue = "true"

var v15MetadataKeys = v15TaskMetadataKeys()

func v15TaskMetadataKeys() map[string]bool {
	keys := make(map[string]bool, len(v14MetadataKeys)+1)
	for key := range v14MetadataKeys {
		keys[key] = true
	}
	keys[v15ReviewMetadataKey] = true
	return keys
}

// The review flag is optional, so schema 15 has the same required metadata as
// schema 14.
var v15RequiredMetadataKeys = v14RequiredMetadataKeys

// ValidateV15TaskRecord validates a complete schema-15 task record.
func ValidateV15TaskRecord(task V15TaskRecord) error {
	return validateV15TaskRecord(task, true)
}

func validateV15TaskRecord(task V15TaskRecord, requireTitle bool) error {
	return validateV14TaskRecord(task.V14TaskRecord, requireTitle)
}

// WriteV15TaskMetadata writes the canonical schema-15 metadata comment. A false
// review flag omits the key entirely, which is what keeps a migrated record
// byte-equal to the schema-14 record it came from.
func WriteV15TaskMetadata(task V15TaskRecord) (string, error) {
	if err := validateV15TaskRecord(task, false); err != nil {
		return "", err
	}
	return wrapTaskMetadata(v15MetadataParts(task)), nil
}

// v15MetadataParts renders the schema-15 metadata fields in canonical order,
// split from the writer above the way v6MetadataParts is split from its own, so
// a later schema appending keys of its own spells this order once rather than
// copying it. Nothing here decides how the parts are wrapped.
func v15MetadataParts(task V15TaskRecord) []string {
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
	if len(task.Dependencies) > 0 {
		parts = append(parts, "depends: "+encodeV14Dependencies(task.Dependencies))
	}
	if task.Review {
		parts = append(parts, v15ReviewMetadataKey+": "+v15ReviewTrue)
	}
	return parts
}

// ParseV15TaskMetadata strictly parses one schema-15 metadata comment.
func ParseV15TaskMetadata(comment string) (V15TaskRecord, error) {
	values, err := parseTaskMetadataFields("v15", comment, v15MetadataKeys, v15RequiredMetadataKeys)
	if err != nil {
		return V15TaskRecord{}, err
	}
	task, err := v15RecordFromMetadataValues(values)
	if err != nil {
		return V15TaskRecord{}, err
	}
	if err := validateV15TaskRecord(task, false); err != nil {
		return V15TaskRecord{}, err
	}
	return task, nil
}

// v15RecordFromMetadataValues decodes the schema-15 metadata fields from the
// already-split key/value pairs, the way v6RecordFromMetadataValues decodes the
// shared ones. It is the kernel a later schema reuses to read the fields it
// inherits, so the nesting from V6TaskRecord up to V15TaskRecord is spelled once
// and the keys it names cannot drift from the ones v15MetadataParts writes. Its
// errors keep saying "v15" for the same reason the v6 kernel keeps saying "v6":
// the field it failed on belongs to that schema whoever is reading it.
func v15RecordFromMetadataValues(values map[string]string) (V15TaskRecord, error) {
	base, err := v6RecordFromMetadataValues(values, deferInstant)
	if err != nil {
		return V15TaskRecord{}, err
	}
	stateChangedAt, err := parseV6Time(values["state_changed"])
	if err != nil {
		return V15TaskRecord{}, fmt.Errorf("parse v15 state_changed: %w", err)
	}
	deferCount := 0
	if value, present := values["defer_count"]; present {
		deferCount, err = strconv.Atoi(value)
		if err != nil {
			return V15TaskRecord{}, fmt.Errorf("parse v15 defer_count: %w", err)
		}
		if deferCount == 0 {
			return V15TaskRecord{}, fmt.Errorf("parse v15 defer_count: value must be positive when present")
		}
	}
	task := V15TaskRecord{
		V14TaskRecord: V14TaskRecord{
			V13TaskRecord: V13TaskRecord{
				V12TaskRecord: V12TaskRecord{
					V11TaskRecord: V11TaskRecord{
						V10TaskRecord: V10TaskRecord{V6TaskRecord: base, StateChangedAt: stateChangedAt},
						Context:       values["context"],
					},
					DeferCount: deferCount,
				},
				DueTime: values["due_time"],
			},
		},
	}
	if raw, present := values["depends"]; present {
		task.Dependencies, err = decodeV14Dependencies(raw)
		if err != nil {
			return V15TaskRecord{}, err
		}
	}
	if raw, present := values[v15ReviewMetadataKey]; present {
		if raw != v15ReviewTrue {
			return V15TaskRecord{}, fmt.Errorf("parse v15 review: value must be %q when present, got %q", v15ReviewTrue, raw)
		}
		task.Review = true
	}
	return task, nil
}
