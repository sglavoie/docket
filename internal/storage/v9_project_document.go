package storage

import (
	"github.com/sglavoie/tt/internal/storage/format"
)

// SchemaVersionV9 renames the durable deferral-date metadata key from "snoozed"
// to "deferred" while retaining schema 8's task fields, tag contract, state
// clock, and generic Markdown headings.
const SchemaVersionV9 = 9

// schemaMarkerV9 pins the exact marker line schema-9 task Markdown carries, so a
// change to the marker format helper cannot silently rewrite what the v11
// migration reads.
const schemaMarkerV9 = "<!-- tt-schema: 9 -->"

// V9ProjectDocument is one project task file in schema 9. The schema-9 change is
// a durable key rather than a field, so the document is an alias of the schema-8
// document, following the schema-7 precedent.
type V9ProjectDocument = V8ProjectDocument

// ValidateV9ProjectDocument validates the schema-8 invariants, which schema 9
// inherits unchanged.
func ValidateV9ProjectDocument(document V9ProjectDocument) error {
	return ValidateV8ProjectDocument(document)
}

// ParseV9ProjectDocument strictly parses schema-9 project Markdown.
func ParseV9ProjectDocument(content string) (V9ProjectDocument, error) {
	scanned, err := scanTaskProjectDocument(content, SchemaVersionV9, "v9")
	if err != nil {
		return V9ProjectDocument{}, err
	}
	document := V9ProjectDocument{ProjectName: scanned.ProjectName}
	for _, block := range scanned.Tasks {
		task, err := format.ParseV9TaskBody(block.Title, block.Body)
		if err != nil {
			return V9ProjectDocument{}, err
		}
		document.Tasks = append(document.Tasks, task)
	}
	if err := ValidateV9ProjectDocument(document); err != nil {
		return V9ProjectDocument{}, err
	}
	return document, nil
}

// WriteV9ProjectDocument renders canonical schema-9 Markdown.
func WriteV9ProjectDocument(document V9ProjectDocument) (string, error) {
	if err := ValidateV9ProjectDocument(document); err != nil {
		return "", err
	}
	records := make([]format.V6TaskRecord, 0, len(document.Tasks))
	for _, task := range document.Tasks {
		records = append(records, task.V6TaskRecord)
	}
	return writeTaskProjectDocument(document.ProjectName, schemaMarkerV9, records, func(i int) (string, error) {
		return format.WriteV9TaskBody(document.Tasks[i])
	})
}

// V9ProjectDocumentFromV8 lifts a schema-8 document to schema 9. The lift is an
// identity — the rename lives entirely in the metadata writer — and it exists so
// a migration call site reads like every other schema boundary.
func V9ProjectDocumentFromV8(document V8ProjectDocument) V9ProjectDocument {
	return document
}
