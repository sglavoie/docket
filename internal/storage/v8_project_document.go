package storage

import (
	"github.com/sglavoie/tt/internal/storage/format"
)

// SchemaVersionV8 adds the durable state clock to every task while retaining
// schema 7's tag contract, task fields, and generic Markdown headings.
const SchemaVersionV8 = 8

// schemaMarkerV8 pins the exact marker line live task Markdown carries, so a
// change to the marker format helper cannot silently rewrite what is on disk.
const schemaMarkerV8 = "<!-- tt-schema: 8 -->"

// V8ProjectDocument is one project task file in schema 8.
type V8ProjectDocument struct {
	ProjectName string
	Tasks       []format.V8TaskRecord
}

// ValidateV8ProjectDocument validates the schema-7 project invariants over the
// shared task fields, then the schema-8 field each task adds.
func ValidateV8ProjectDocument(document V8ProjectDocument) error {
	if err := ValidateV7ProjectDocument(v7ProjectDocumentFromV8(document)); err != nil {
		return err
	}
	for _, task := range document.Tasks {
		if err := format.ValidateV8TaskRecord(task); err != nil {
			return err
		}
	}
	return nil
}

// ParseV8ProjectDocument strictly parses schema-8 project Markdown.
func ParseV8ProjectDocument(content string) (V8ProjectDocument, error) {
	scanned, err := scanTaskProjectDocument(content, SchemaVersionV8, "v8")
	if err != nil {
		return V8ProjectDocument{}, err
	}
	document := V8ProjectDocument{ProjectName: scanned.ProjectName}
	for _, block := range scanned.Tasks {
		task, err := format.ParseV8TaskBody(block.Title, block.Body)
		if err != nil {
			return V8ProjectDocument{}, err
		}
		document.Tasks = append(document.Tasks, task)
	}
	if err := ValidateV8ProjectDocument(document); err != nil {
		return V8ProjectDocument{}, err
	}
	return document, nil
}

// WriteV8ProjectDocument renders canonical schema-8 Markdown.
func WriteV8ProjectDocument(document V8ProjectDocument) (string, error) {
	if err := ValidateV8ProjectDocument(document); err != nil {
		return "", err
	}
	records := make([]format.V6TaskRecord, 0, len(document.Tasks))
	for _, task := range document.Tasks {
		records = append(records, task.V6TaskRecord)
	}
	return writeTaskProjectDocument(document.ProjectName, schemaMarkerV8, records, func(i int) (string, error) {
		return format.WriteV8TaskBody(document.Tasks[i])
	})
}

// V8ProjectDocumentFromV7 lifts a schema-7 document to schema 8, backfilling
// each task's state clock from UpdatedAt.
func V8ProjectDocumentFromV7(document V7ProjectDocument) V8ProjectDocument {
	lifted := V8ProjectDocument{ProjectName: document.ProjectName}
	for _, task := range document.Tasks {
		lifted.Tasks = append(lifted.Tasks, format.V8TaskRecordFromV7(task))
	}
	return lifted
}

func v7ProjectDocumentFromV8(document V8ProjectDocument) V7ProjectDocument {
	projected := V7ProjectDocument{ProjectName: document.ProjectName}
	for _, task := range document.Tasks {
		projected.Tasks = append(projected.Tasks, task.V6TaskRecord)
	}
	return projected
}
