package storage

import "github.com/sglavoie/docket/internal/storage/format"

// SchemaVersionV11 adds one optional, single-valued context to each task.
const SchemaVersionV11 = 11

// schemaMarkerV11 pins the exact marker line live task Markdown carries.
const schemaMarkerV11 = "<!-- tt-schema: 11 -->"

// V11ProjectDocument is one project task file in schema 11. It is a real
// struct because the task record diverges from schema 10 for the first time.
type V11ProjectDocument struct {
	ProjectName string
	Tasks       []format.V11TaskRecord
}

// ValidateV11ProjectDocument validates shared project invariants plus every
// schema-11 task record.
func ValidateV11ProjectDocument(document V11ProjectDocument) error {
	projected := v7ProjectDocumentFromV8(v8ProjectDocumentFromV11(document))
	return validateTaskProjectDocument(projected.ProjectName, projected.Tasks, func(i int) error {
		return format.ValidateV11TaskRecord(document.Tasks[i])
	})
}

// ParseV11ProjectDocument strictly parses schema-11 project Markdown.
func ParseV11ProjectDocument(content string) (V11ProjectDocument, error) {
	scanned, err := scanTaskProjectDocument(content, SchemaVersionV11, "v11")
	if err != nil {
		return V11ProjectDocument{}, err
	}
	document := V11ProjectDocument{ProjectName: scanned.ProjectName}
	for _, block := range scanned.Tasks {
		task, err := format.ParseV11TaskBody(block.Title, block.Body)
		if err != nil {
			return V11ProjectDocument{}, err
		}
		document.Tasks = append(document.Tasks, task)
	}
	if err := ValidateV11ProjectDocument(document); err != nil {
		return V11ProjectDocument{}, err
	}
	return document, nil
}

// WriteV11ProjectDocument renders canonical schema-11 Markdown.
func WriteV11ProjectDocument(document V11ProjectDocument) (string, error) {
	if err := ValidateV11ProjectDocument(document); err != nil {
		return "", err
	}
	records := make([]format.V6TaskRecord, 0, len(document.Tasks))
	for _, task := range document.Tasks {
		records = append(records, task.V6TaskRecord)
	}
	return writeTaskProjectDocument(document.ProjectName, schemaMarkerV11, records, func(i int) (string, error) {
		return format.WriteV11TaskBody(document.Tasks[i])
	})
}

// V11ProjectDocumentFromV10 lifts a schema-10 document without changing any
// existing value. The task slice is rebuilt because its element type changed.
func V11ProjectDocumentFromV10(document V10ProjectDocument) V11ProjectDocument {
	lifted := V11ProjectDocument{ProjectName: document.ProjectName}
	if len(document.Tasks) > 0 {
		lifted.Tasks = make([]format.V11TaskRecord, 0, len(document.Tasks))
	}
	for _, task := range document.Tasks {
		lifted.Tasks = append(lifted.Tasks, format.V11TaskRecord{V10TaskRecord: task})
	}
	return lifted
}

func v8ProjectDocumentFromV11(document V11ProjectDocument) V8ProjectDocument {
	projected := V8ProjectDocument{ProjectName: document.ProjectName}
	if len(document.Tasks) > 0 {
		projected.Tasks = make([]format.V8TaskRecord, 0, len(document.Tasks))
	}
	for _, task := range document.Tasks {
		projected.Tasks = append(projected.Tasks, task.V10TaskRecord)
	}
	return projected
}
