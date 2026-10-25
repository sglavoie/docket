package storage

import (
	"fmt"
	"time"

	"github.com/sglavoie/docket/internal/storage/format"
)

// SchemaVersionV10 carries the deferral value as a canonical RFC3339Nano
// instant rather than a calendar day, which is what makes a deferral that
// expires later the same day representable. Schema 9's fields, metadata keys,
// tag contract, state clock, and generic Markdown headings are unchanged.
const SchemaVersionV10 = 10

// schemaMarkerV10 pins the exact marker line live task Markdown carries, so a
// change to the marker format helper cannot silently rewrite what is on disk.
const schemaMarkerV10 = "<!-- tt-schema: 10 -->"

// V10ProjectDocument is one project task file in schema 10. The schema-10
// change is a durable value layout rather than a field, so the document is an
// alias of the schema-9 document, following the schema-9 precedent.
type V10ProjectDocument = V9ProjectDocument

// ValidateV10ProjectDocument validates the shared project invariants with the
// schema-10 task validator. It cannot delegate to ValidateV9ProjectDocument the
// way schema 9 delegates to schema 8: that chain checks the deferral value
// against the calendar-day layout, which every schema-10 instant fails.
func ValidateV10ProjectDocument(document V10ProjectDocument) error {
	projected := v7ProjectDocumentFromV8(document)
	return validateTaskProjectDocument(projected.ProjectName, projected.Tasks, func(i int) error {
		return format.ValidateV10TaskRecord(document.Tasks[i])
	})
}

// ParseV10ProjectDocument strictly parses schema-10 project Markdown.
func ParseV10ProjectDocument(content string) (V10ProjectDocument, error) {
	scanned, err := scanTaskProjectDocument(content, SchemaVersionV10, "v10")
	if err != nil {
		return V10ProjectDocument{}, err
	}
	document := V10ProjectDocument{ProjectName: scanned.ProjectName}
	for _, block := range scanned.Tasks {
		task, err := format.ParseV10TaskBody(block.Title, block.Body)
		if err != nil {
			return V10ProjectDocument{}, err
		}
		document.Tasks = append(document.Tasks, task)
	}
	if err := ValidateV10ProjectDocument(document); err != nil {
		return V10ProjectDocument{}, err
	}
	return document, nil
}

// WriteV10ProjectDocument renders canonical schema-10 Markdown.
func WriteV10ProjectDocument(document V10ProjectDocument) (string, error) {
	if err := ValidateV10ProjectDocument(document); err != nil {
		return "", err
	}
	records := make([]format.V6TaskRecord, 0, len(document.Tasks))
	for _, task := range document.Tasks {
		records = append(records, task.V6TaskRecord)
	}
	return writeTaskProjectDocument(document.ProjectName, schemaMarkerV10, records, func(i int) (string, error) {
		return format.WriteV10TaskBody(document.Tasks[i])
	})
}

// V10ProjectDocumentFromV9 lifts a schema-9 document to schema 10 by rewriting
// each deferral date as the instant that day began in loc. loc is the location
// the bare date was always implicitly read in, so the lift preserves the exact
// moment every existing deferral already expired at. The lifted document is a
// fresh value: unlike the identity lifts before it this one rewrites a field,
// so the caller's pre-lift document has to stay intact.
func V10ProjectDocumentFromV9(document V9ProjectDocument, loc *time.Location) (V10ProjectDocument, error) {
	lifted := V10ProjectDocument{ProjectName: document.ProjectName}
	for _, task := range document.Tasks {
		if task.SnoozedUntil != "" {
			day, err := time.ParseInLocation("2006-01-02", task.SnoozedUntil, loc)
			if err != nil {
				return V10ProjectDocument{}, fmt.Errorf("lift v10 task %q deferral date %q: %w", task.ID, task.SnoozedUntil, err)
			}
			task.SnoozedUntil = day.Format(time.RFC3339Nano)
		}
		lifted.Tasks = append(lifted.Tasks, task)
	}
	return lifted, nil
}
