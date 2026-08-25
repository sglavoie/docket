package storage

import "github.com/sglavoie/tt/internal/storage/format"

// SchemaVersionV13 adds the optional due-date clock time (due_time).
const SchemaVersionV13 = 13

// schemaMarkerV13 pins the exact marker line live task Markdown carries.
const schemaMarkerV13 = "<!-- tt-schema: 13 -->"

// V13ProjectDocument is one project task file in schema 13.
type V13ProjectDocument struct {
	ProjectName string
	Tasks       []format.V13TaskRecord
}

// ValidateV13ProjectDocument validates shared project invariants plus every
// schema-13 task record.
func ValidateV13ProjectDocument(document V13ProjectDocument) error {
	projected := v7ProjectDocumentFromV8(v8ProjectDocumentFromV11(v11ProjectDocumentFromV12(v12ProjectDocumentFromV13(document))))
	return validateTaskProjectDocument(projected.ProjectName, projected.Tasks, func(i int) error {
		return format.ValidateV13TaskRecord(document.Tasks[i])
	})
}

// ParseV13ProjectDocument strictly parses schema-13 project Markdown.
func ParseV13ProjectDocument(content string) (V13ProjectDocument, error) {
	scanned, err := scanTaskProjectDocument(content, SchemaVersionV13, "v13")
	if err != nil {
		return V13ProjectDocument{}, err
	}
	document := V13ProjectDocument{ProjectName: scanned.ProjectName}
	for _, block := range scanned.Tasks {
		task, err := format.ParseV13TaskBody(block.Title, block.Body)
		if err != nil {
			return V13ProjectDocument{}, err
		}
		document.Tasks = append(document.Tasks, task)
	}
	if err := ValidateV13ProjectDocument(document); err != nil {
		return V13ProjectDocument{}, err
	}
	return document, nil
}

// WriteV13ProjectDocument renders canonical schema-13 Markdown.
func WriteV13ProjectDocument(document V13ProjectDocument) (string, error) {
	if err := ValidateV13ProjectDocument(document); err != nil {
		return "", err
	}
	records := make([]format.V6TaskRecord, 0, len(document.Tasks))
	for _, task := range document.Tasks {
		records = append(records, task.V6TaskRecord)
	}
	return writeTaskProjectDocument(document.ProjectName, schemaMarkerV13, records, func(i int) (string, error) {
		return format.WriteV13TaskBody(document.Tasks[i])
	})
}

// V13ProjectDocumentFromV12 lifts a schema-12 document without changing any
// existing value. Every migrated task starts with no due_time, which is exactly
// a date-only due date. The task slice is rebuilt because its element type
// changed.
func V13ProjectDocumentFromV12(document V12ProjectDocument) V13ProjectDocument {
	lifted := V13ProjectDocument{ProjectName: document.ProjectName}
	if len(document.Tasks) > 0 {
		lifted.Tasks = make([]format.V13TaskRecord, 0, len(document.Tasks))
	}
	for _, task := range document.Tasks {
		lifted.Tasks = append(lifted.Tasks, format.V13TaskRecord{V12TaskRecord: task})
	}
	return lifted
}

func v12ProjectDocumentFromV13(document V13ProjectDocument) V12ProjectDocument {
	projected := V12ProjectDocument{ProjectName: document.ProjectName}
	if len(document.Tasks) > 0 {
		projected.Tasks = make([]format.V12TaskRecord, 0, len(document.Tasks))
	}
	for _, task := range document.Tasks {
		projected.Tasks = append(projected.Tasks, task.V12TaskRecord)
	}
	return projected
}
