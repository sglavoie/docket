package storage

import "github.com/sglavoie/tt/internal/storage/format"

// SchemaVersionV12 adds the optional task deferral count.
const SchemaVersionV12 = 12

// schemaMarkerV12 pins the exact marker line live task Markdown carries.
const schemaMarkerV12 = "<!-- tt-schema: 12 -->"

// V12ProjectDocument is one project task file in schema 12.
type V12ProjectDocument struct {
	ProjectName string
	Tasks       []format.V12TaskRecord
}

// ValidateV12ProjectDocument validates shared project invariants plus every
// schema-12 task record.
func ValidateV12ProjectDocument(document V12ProjectDocument) error {
	projected := v7ProjectDocumentFromV8(v8ProjectDocumentFromV11(v11ProjectDocumentFromV12(document)))
	return validateTaskProjectDocument(projected.ProjectName, projected.Tasks, func(i int) error {
		return format.ValidateV12TaskRecord(document.Tasks[i])
	})
}

// ParseV12ProjectDocument strictly parses schema-12 project Markdown.
func ParseV12ProjectDocument(content string) (V12ProjectDocument, error) {
	scanned, err := scanTaskProjectDocument(content, SchemaVersionV12, "v12")
	if err != nil {
		return V12ProjectDocument{}, err
	}
	document := V12ProjectDocument{ProjectName: scanned.ProjectName}
	for _, block := range scanned.Tasks {
		task, err := format.ParseV12TaskBody(block.Title, block.Body)
		if err != nil {
			return V12ProjectDocument{}, err
		}
		document.Tasks = append(document.Tasks, task)
	}
	if err := ValidateV12ProjectDocument(document); err != nil {
		return V12ProjectDocument{}, err
	}
	return document, nil
}

// WriteV12ProjectDocument renders canonical schema-12 Markdown.
func WriteV12ProjectDocument(document V12ProjectDocument) (string, error) {
	if err := ValidateV12ProjectDocument(document); err != nil {
		return "", err
	}
	records := make([]format.V6TaskRecord, 0, len(document.Tasks))
	for _, task := range document.Tasks {
		records = append(records, task.V6TaskRecord)
	}
	return writeTaskProjectDocument(document.ProjectName, schemaMarkerV12, records, func(i int) (string, error) {
		return format.WriteV12TaskBody(document.Tasks[i])
	})
}

// V12ProjectDocumentFromV11 lifts a schema-11 document without changing any
// existing value. The task slice is rebuilt because its element type changed.
func V12ProjectDocumentFromV11(document V11ProjectDocument) V12ProjectDocument {
	lifted := V12ProjectDocument{ProjectName: document.ProjectName}
	if len(document.Tasks) > 0 {
		lifted.Tasks = make([]format.V12TaskRecord, 0, len(document.Tasks))
	}
	for _, task := range document.Tasks {
		lifted.Tasks = append(lifted.Tasks, format.V12TaskRecord{V11TaskRecord: task})
	}
	return lifted
}

func v11ProjectDocumentFromV12(document V12ProjectDocument) V11ProjectDocument {
	projected := V11ProjectDocument{ProjectName: document.ProjectName}
	if len(document.Tasks) > 0 {
		projected.Tasks = make([]format.V11TaskRecord, 0, len(document.Tasks))
	}
	for _, task := range document.Tasks {
		projected.Tasks = append(projected.Tasks, task.V11TaskRecord)
	}
	return projected
}
