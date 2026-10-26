package storage

import "github.com/sglavoie/docket/internal/storage/format"

// SchemaVersionV16 adds the optional retained resume note.
const SchemaVersionV16 = 16

// schemaMarkerV16 pins the exact marker line live task Markdown carries.
const schemaMarkerV16 = "<!-- tt-schema: 16 -->"

// V16ProjectDocument is one project task file in schema 16.
type V16ProjectDocument struct {
	ProjectName string
	Tasks       []format.V16TaskRecord
}

// ValidateV16ProjectDocument validates shared project invariants plus every
// schema-16 task record.
func ValidateV16ProjectDocument(document V16ProjectDocument) error {
	// The shared invariants read only the schema-6 record each task embeds, so it
	// is taken directly rather than through a copy per schema in between.
	records := make([]format.V6TaskRecord, 0, len(document.Tasks))
	for _, task := range document.Tasks {
		records = append(records, task.V6TaskRecord)
	}
	return validateTaskProjectDocument(document.ProjectName, records, func(i int) error {
		return format.ValidateV16TaskRecord(document.Tasks[i])
	})
}

// ParseV16ProjectDocument strictly parses schema-16 project Markdown.
func ParseV16ProjectDocument(content string) (V16ProjectDocument, error) {
	scanned, err := scanTaskProjectDocument(content, SchemaVersionV16, "v16")
	if err != nil {
		return V16ProjectDocument{}, err
	}
	document := V16ProjectDocument{ProjectName: scanned.ProjectName}
	for _, block := range scanned.Tasks {
		task, err := format.ParseV16TaskBody(block.Title, block.Body)
		if err != nil {
			return V16ProjectDocument{}, err
		}
		document.Tasks = append(document.Tasks, task)
	}
	if err := ValidateV16ProjectDocument(document); err != nil {
		return V16ProjectDocument{}, err
	}
	return document, nil
}

// WriteV16ProjectDocument renders canonical schema-16 Markdown.
func WriteV16ProjectDocument(document V16ProjectDocument) (string, error) {
	if err := ValidateV16ProjectDocument(document); err != nil {
		return "", err
	}
	records := make([]format.V6TaskRecord, 0, len(document.Tasks))
	for _, task := range document.Tasks {
		records = append(records, task.V6TaskRecord)
	}
	return writeTaskProjectDocument(document.ProjectName, schemaMarkerV16, records, func(i int) (string, error) {
		return format.WriteV16TaskBody(document.Tasks[i])
	})
}

// V16ProjectDocumentFromV15 lifts a schema-15 document without changing any
// existing value. No retired field carried resume text into this boundary, so
// every migrated task starts with no note, which is exactly an omitted RESUME
// block. The task slice is rebuilt because its element type changed.
func V16ProjectDocumentFromV15(document V15ProjectDocument) V16ProjectDocument {
	lifted := V16ProjectDocument{ProjectName: document.ProjectName}
	if len(document.Tasks) > 0 {
		lifted.Tasks = make([]format.V16TaskRecord, 0, len(document.Tasks))
	}
	for _, task := range document.Tasks {
		lifted.Tasks = append(lifted.Tasks, format.V16TaskRecord{V15TaskRecord: task})
	}
	return lifted
}
