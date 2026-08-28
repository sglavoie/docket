package storage

import "github.com/sglavoie/tt/internal/storage/format"

// SchemaVersionV14 adds the optional ordered list of soft task dependencies.
const SchemaVersionV14 = 14

// schemaMarkerV14 pins the exact marker line live task Markdown carries.
const schemaMarkerV14 = "<!-- tt-schema: 14 -->"

// V14ProjectDocument is one project task file in schema 14.
type V14ProjectDocument struct {
	ProjectName string
	Tasks       []format.V14TaskRecord
}

// ValidateV14ProjectDocument validates shared project invariants plus every
// schema-14 task record.
func ValidateV14ProjectDocument(document V14ProjectDocument) error {
	projected := v7ProjectDocumentFromV8(v8ProjectDocumentFromV11(v11ProjectDocumentFromV12(v12ProjectDocumentFromV13(v13ProjectDocumentFromV14(document)))))
	return validateTaskProjectDocument(projected.ProjectName, projected.Tasks, func(i int) error {
		return format.ValidateV14TaskRecord(document.Tasks[i])
	})
}

// ParseV14ProjectDocument strictly parses schema-14 project Markdown.
func ParseV14ProjectDocument(content string) (V14ProjectDocument, error) {
	scanned, err := scanTaskProjectDocument(content, SchemaVersionV14, "v14")
	if err != nil {
		return V14ProjectDocument{}, err
	}
	document := V14ProjectDocument{ProjectName: scanned.ProjectName}
	for _, block := range scanned.Tasks {
		task, err := format.ParseV14TaskBody(block.Title, block.Body)
		if err != nil {
			return V14ProjectDocument{}, err
		}
		document.Tasks = append(document.Tasks, task)
	}
	if err := ValidateV14ProjectDocument(document); err != nil {
		return V14ProjectDocument{}, err
	}
	return document, nil
}

// WriteV14ProjectDocument renders canonical schema-14 Markdown.
func WriteV14ProjectDocument(document V14ProjectDocument) (string, error) {
	if err := ValidateV14ProjectDocument(document); err != nil {
		return "", err
	}
	records := make([]format.V6TaskRecord, 0, len(document.Tasks))
	for _, task := range document.Tasks {
		records = append(records, task.V6TaskRecord)
	}
	return writeTaskProjectDocument(document.ProjectName, schemaMarkerV14, records, func(i int) (string, error) {
		return format.WriteV14TaskBody(document.Tasks[i])
	})
}

// V14ProjectDocumentFromV13 lifts a schema-13 document without changing any
// existing value. Every migrated task starts with no dependencies, which is
// exactly an omitted depends key. The task slice is rebuilt because its element
// type changed.
func V14ProjectDocumentFromV13(document V13ProjectDocument) V14ProjectDocument {
	lifted := V14ProjectDocument{ProjectName: document.ProjectName}
	if len(document.Tasks) > 0 {
		lifted.Tasks = make([]format.V14TaskRecord, 0, len(document.Tasks))
	}
	for _, task := range document.Tasks {
		lifted.Tasks = append(lifted.Tasks, format.V14TaskRecord{V13TaskRecord: task})
	}
	return lifted
}

func v13ProjectDocumentFromV14(document V14ProjectDocument) V13ProjectDocument {
	projected := V13ProjectDocument{ProjectName: document.ProjectName}
	if len(document.Tasks) > 0 {
		projected.Tasks = make([]format.V13TaskRecord, 0, len(document.Tasks))
	}
	for _, task := range document.Tasks {
		projected.Tasks = append(projected.Tasks, task.V13TaskRecord)
	}
	return projected
}
