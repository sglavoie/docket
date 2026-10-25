package format

import (
	"github.com/sglavoie/docket/internal/models"
)

// V16TaskRecord is the complete schema-16 storage shape: the schema-15 fields
// plus the retained resume note. The note answers "where was I" for one task and
// is kept until it is explicitly cleared, so it is durable state rather than a
// comment about a moment, which is why it is a field of its own and not the
// newest entry in the comment list.
//
// The note is written as a body block rather than a metadata key. Metadata is
// one comma-separated line, so a value carrying arbitrary prose would have to be
// escaped against that line's grammar; the block grammar already escapes prose
// for the description and the comments, and the note is prose.
//
// The block is optional and written only when the note is non-empty, so a task
// with no note stays byte-identical to its schema-15 predecessor. That is the
// same additive discipline depends, due_time, and review were added under.
type V16TaskRecord struct {
	V15TaskRecord
	ResumeNote string
}

// ValidateV16TaskRecord validates a complete schema-16 task record.
func ValidateV16TaskRecord(task V16TaskRecord) error {
	return validateV16TaskRecord(task, true)
}

// validateV16TaskRecord validates the schema-15 fields it inherits and then the
// one field schema 16 adds. The note is bounded by the model rather than here so
// the storage writer and every later editing surface refuse exactly the same
// values.
func validateV16TaskRecord(task V16TaskRecord, requireTitle bool) error {
	if err := validateV15TaskRecord(task.V15TaskRecord, requireTitle); err != nil {
		return err
	}
	return models.ValidateResumeNote(task.ResumeNote)
}
